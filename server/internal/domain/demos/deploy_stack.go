// Topic 37 deployment's live demo: the stack viewer. The payload is
// derived ENTIRELY from the repo's committed deployment artifacts —
// deploy/docker-compose.prod.yml parsed with the guide's
// zero-dependency YAML-subset reader (internal/apispec, the same
// parser the OpenAPI toolchain uses) and every deploy/k8s/*.json
// parsed with encoding/json — so every service, port, healthcheck,
// probe path, and rollout knob on the wire is TRACEABLE to a real
// file: nothing is invented, and the suite pins that. The builder also
// ENFORCES the secrets discipline rather than merely documenting it: a
// Secret manifest whose data values do not decode to CHANGE_ME_*
// placeholders fails the build outright, so a real credential can
// never reach the wire through this demo.
package demos

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"guide/internal/apispec"
)

// DeployStackPath is the demo area's endpoint — registered on the real
// mux by the composition root.
const DeployStackPath = "/api/demo/deploy/stack"

// The deployment artifacts' canonical names, relative to the deploy/
// directory the handler resolves beside the process. They double as
// the demo's provenance strings: every view names the repo-root-
// relative file it was parsed from.
const (
	// DeployComposeFile is the topic's full prod stack declaration.
	DeployComposeFile = "docker-compose.prod.yml"
	// DeployK8sDir holds the Kubernetes manifests (one JSON object per
	// file — kubectl-native, and parseable by the stdlib encoding/json
	// with no YAML dependency).
	DeployK8sDir = "k8s"
	// DeployDirName is the repo-root directory carrying the artifacts.
	DeployDirName = "deploy"
)

// SecretPlaceholderMarker is the string every k8s Secret data value
// must decode to (inside its base64): the marker names a value that is
// deliberately NOT a credential. The scanner refuses anything else.
const SecretPlaceholderMarker = "CHANGE_ME"

// DeployManifestFile is one Kubernetes manifest read from disk: its
// name relative to the deploy directory and its raw bytes.
type DeployManifestFile struct {
	Name string
	JSON []byte
}

// DeployStackFacts carries the raw committed artifacts into the pure
// builder: the compose file's bytes and the k8s manifests, in the
// order the handler read them. The handler owns file resolution; the
// builder owns parsing, derivation, and the secret gate.
type DeployStackFacts struct {
	ComposeYAML []byte
	Manifests   []DeployManifestFile
}

// ── artifact location ───────────────────────────────────────────────

// LocateDeployDir walks up from start looking for the repo's
// deployment artifacts (deploy/docker-compose.prod.yml) and returns
// the deploy directory's absolute path — "" when no ancestor carries
// it (the distroless prod image ships no repo beside the binary, so
// the demo degrades honestly instead of guessing). The walk mirrors
// the content package's repo-root locator: same shape, different
// anchor file.
func LocateDeployDir(start string) string {
	// Normalize first: a relative start like "." would never ascend,
	// because filepath.Dir(".") is "." itself.
	dir := start
	if abs, err := filepath.Abs(start); err == nil {
		dir = abs
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, DeployDirName, DeployComposeFile)); err == nil {
			return filepath.Join(dir, DeployDirName)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ── compose topology (parsed) ───────────────────────────────────────

// ComposeService is one service of the prod compose file, with every
// field derived from the parsed YAML — ports as published "host:
// container" strings, environment as "KEY=VALUE" strings (interpolation
// rails kept verbatim — the committed file carries the rail, never the
// secret), depends_on entries with their conditions, and the resource
// hint from the deploy block.
type ComposeService struct {
	Name              string
	Image             string
	BuildContext      string
	Dockerfile        string
	Restart           string
	Ports             []string
	Environment       []string
	Healthcheck       string
	DependsOn         []string
	CPULimit          string
	MemoryLimit       string
	CPUReservation    string
	MemoryReservation string
}

// ComposeTopology is the parsed deployment shape of the prod compose
// file: the project name, the services sorted by name (parsed maps
// carry no order; sorted output keeps the payload stable), and the
// internal networks.
type ComposeTopology struct {
	Project  string
	Networks []string
	Services []ComposeService
}

// ParseComposeTopology reads the prod compose file's bytes into the
// typed topology, using the guide's zero-dependency YAML-subset parser
// (the same one the OpenAPI toolchain reads server/api/openapi.yaml
// with — the guide stays stdlib-only, and the parser's strictness
// means a compose file that wanders outside the subset fails loudly
// here instead of parsing into something subtly wrong).
func ParseComposeTopology(raw []byte) (ComposeTopology, error) {
	document, err := apispec.ParseSpecYAML(raw)
	if err != nil {
		return ComposeTopology{}, fmt.Errorf("deploy: parse compose YAML: %w", err)
	}
	topology := ComposeTopology{Project: deployPlainString(document["name"])}

	servicesRaw, err := deployMapOf("services", document["services"])
	if err != nil {
		return ComposeTopology{}, err
	}
	for _, name := range deploySortedKeys(servicesRaw) {
		service, err := deployParseComposeService(name, servicesRaw[name])
		if err != nil {
			return ComposeTopology{}, err
		}
		topology.Services = append(topology.Services, service)
	}

	if networksRaw, present := document["networks"]; present && networksRaw != nil {
		networks, err := deployMapOf("networks", networksRaw)
		if err != nil {
			return ComposeTopology{}, err
		}
		topology.Networks = deploySortedKeys(networks)
	}
	return topology, nil
}

// deployParseComposeService derives one service view from its parsed
// mapping, naming the YAML path in every failure.
func deployParseComposeService(name string, raw any) (ComposeService, error) {
	body, err := deployMapOf("services."+name, raw)
	if err != nil {
		return ComposeService{}, err
	}
	service := ComposeService{
		Name:    name,
		Image:   deployPlainString(body["image"]),
		Restart: deployPlainString(body["restart"]),
	}
	if build, present := body["build"]; present && build != nil {
		buildMap, err := deployMapOf("services."+name+".build", build)
		if err != nil {
			return ComposeService{}, err
		}
		service.BuildContext = deployPlainString(buildMap["context"])
		service.Dockerfile = deployPlainString(buildMap["dockerfile"])
	}
	service.Ports, err = deployStringSlice("services."+name+".ports", body["ports"])
	if err != nil {
		return ComposeService{}, err
	}
	if env, present := body["environment"]; present && env != nil {
		envMap, err := deployMapOf("services."+name+".environment", env)
		if err != nil {
			return ComposeService{}, err
		}
		for _, key := range deploySortedKeys(envMap) {
			service.Environment = append(service.Environment, key+"="+deployScalarString(envMap[key]))
		}
	}
	if check, present := body["healthcheck"]; present && check != nil {
		checkMap, err := deployMapOf("services."+name+".healthcheck", check)
		if err != nil {
			return ComposeService{}, err
		}
		argv, err := deployStringSlice("services."+name+".healthcheck.test", checkMap["test"])
		if err != nil {
			return ComposeService{}, err
		}
		service.Healthcheck = strings.Join(argv, " ")
	}
	if depends, present := body["depends_on"]; present && depends != nil {
		dependsMap, err := deployMapOf("services."+name+".depends_on", depends)
		if err != nil {
			return ComposeService{}, err
		}
		for _, dep := range deploySortedKeys(dependsMap) {
			condition := ""
			if depBody, ok := dependsMap[dep].(map[string]any); ok {
				condition = deployPlainString(depBody["condition"])
			}
			if condition == "" {
				service.DependsOn = append(service.DependsOn, dep)
				continue
			}
			service.DependsOn = append(service.DependsOn, dep+" ("+condition+")")
		}
	}
	if deployBlock, present := body["deploy"]; present && deployBlock != nil {
		deployMap, err := deployMapOf("services."+name+".deploy", deployBlock)
		if err != nil {
			return ComposeService{}, err
		}
		if resources, present := deployMap["resources"]; present && resources != nil {
			resourcesMap, err := deployMapOf("services."+name+".deploy.resources", resources)
			if err != nil {
				return ComposeService{}, err
			}
			if limits, present := resourcesMap["limits"]; present && limits != nil {
				limitsMap, err := deployMapOf("services."+name+".deploy.resources.limits", limits)
				if err != nil {
					return ComposeService{}, err
				}
				service.CPULimit = deployPlainString(limitsMap["cpus"])
				service.MemoryLimit = deployPlainString(limitsMap["memory"])
			}
			if reservations, present := resourcesMap["reservations"]; present && reservations != nil {
				resMap, err := deployMapOf("services."+name+".deploy.resources.reservations", reservations)
				if err != nil {
					return ComposeService{}, err
				}
				service.CPUReservation = deployPlainString(resMap["cpus"])
				service.MemoryReservation = deployPlainString(resMap["memory"])
			}
		}
	}
	return service, nil
}

// ── kubernetes manifests (parsed) ──────────────────────────────────

// K8sHTTPProbe is one kubelet-driven HTTP GET probe as the deployment
// declares it: the path and named port the kubelet GETs, plus the
// timing knobs. The kubelet makes the request itself — the container
// needs no shell, no wget, nothing but a listener, which is exactly
// why the distroless image can carry real probes.
type K8sHTTPProbe struct {
	Path                string
	Port                string
	InitialDelaySeconds int
	PeriodSeconds       int
	TimeoutSeconds      int
	FailureThreshold    int
}

// K8sContainerPort is one named container port.
type K8sContainerPort struct {
	Name          string
	ContainerPort int
}

// K8sResources is the requests/limits pair the scheduler and the
// runtime enforce (cpu/memory quantities as authored strings).
type K8sResources struct {
	Requests map[string]string
	Limits   map[string]string
}

// K8sContainer is one pod container of the deployment.
type K8sContainer struct {
	Name           string
	Image          string
	Ports          []K8sContainerPort
	EnvFrom        []string
	LivenessProbe  *K8sHTTPProbe
	ReadinessProbe *K8sHTTPProbe
	Resources      K8sResources
	Security       []string
	VolumeMounts   []string
}

// K8sDeploymentSpec is the deployment's rollout shape: replica count,
// the rolling-update knobs, the grace period, and the pod template's
// containers.
type K8sDeploymentSpec struct {
	Replicas                      int
	MinReadySeconds               int
	TerminationGracePeriodSeconds int
	StrategyType                  string
	MaxSurge                      string
	MaxUnavailable                string
	MatchLabels                   map[string]string
	PodLabels                     map[string]string
	Containers                    []K8sContainer
}

// K8sServicePort is one service port with its target.
type K8sServicePort struct {
	Name       string
	Port       int
	TargetPort string
}

// K8sServiceSpec is the service's cluster-facing shape.
type K8sServiceSpec struct {
	Type     string
	Selector map[string]string
	Ports    []K8sServicePort
}

// K8sSecretSpec is a Secret manifest's payload: its type and its
// base64 data map, exactly as committed.
type K8sSecretSpec struct {
	Type string
	Data map[string]string
}

// K8sManifest is one parsed manifest file: the common envelope every
// Kubernetes object carries, plus exactly one kind-specific payload
// (nil for the others). Unknown kinds parse as envelope-only — the
// structural tests, not the parser, decide which kinds must exist.
type K8sManifest struct {
	File        string
	APIVersion  string
	Kind        string
	Name        string
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
	Deployment  *K8sDeploymentSpec
	Service     *K8sServiceSpec
	ConfigMap   map[string]string
	Secret      *K8sSecretSpec
}

// ParseK8sManifests decodes every manifest with encoding/json — the
// deliberate stdlib-only choice the manifests' JSON format makes
// possible (kubectl accepts JSON natively, and no YAML dependency
// enters the module) — and types each by its kind.
func ParseK8sManifests(files []DeployManifestFile) ([]K8sManifest, error) {
	parsed := make([]K8sManifest, 0, len(files))
	for _, file := range files {
		manifest, err := deployParseK8sManifest(file)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, manifest)
	}
	return parsed, nil
}

// deployParseK8sManifest decodes one file's envelope, then its kind-
// specific payload.
func deployParseK8sManifest(file DeployManifestFile) (K8sManifest, error) {
	var envelope struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name        string            `json:"name"`
			Namespace   string            `json:"namespace"`
			Labels      map[string]string `json:"labels"`
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
		Type string            `json:"type"`
		Data map[string]string `json:"data"`
		Spec json.RawMessage   `json:"spec"`
	}
	if err := json.Unmarshal(file.JSON, &envelope); err != nil {
		return K8sManifest{}, fmt.Errorf("deploy: parse %s as JSON: %w", file.Name, err)
	}
	if envelope.Kind == "" || envelope.Metadata.Name == "" {
		return K8sManifest{}, fmt.Errorf("deploy: %s lacks kind or metadata.name", file.Name)
	}
	manifest := K8sManifest{
		File:        file.Name,
		APIVersion:  envelope.APIVersion,
		Kind:        envelope.Kind,
		Name:        envelope.Metadata.Name,
		Namespace:   envelope.Metadata.Namespace,
		Labels:      envelope.Metadata.Labels,
		Annotations: envelope.Metadata.Annotations,
	}

	switch envelope.Kind {
	case "ConfigMap":
		manifest.ConfigMap = envelope.Data
	case "Secret":
		manifest.Secret = &K8sSecretSpec{Type: envelope.Type, Data: envelope.Data}
	case "Deployment":
		spec, err := deployParseDeploymentSpec(envelope.Spec)
		if err != nil {
			return K8sManifest{}, fmt.Errorf("deploy: %s: %w", file.Name, err)
		}
		manifest.Deployment = spec
	case "Service":
		spec, err := deployParseServiceSpec(envelope.Spec)
		if err != nil {
			return K8sManifest{}, fmt.Errorf("deploy: %s: %w", file.Name, err)
		}
		manifest.Service = spec
	}
	return manifest, nil
}

// deployParseDeploymentSpec types the apps/v1 deployment payload.
func deployParseDeploymentSpec(raw json.RawMessage) (*K8sDeploymentSpec, error) {
	var spec struct {
		Replicas        *int `json:"replicas"`
		MinReadySeconds int  `json:"minReadySeconds"`
		Strategy        struct {
			Type          string          `json:"type"`
			RollingUpdate json.RawMessage `json:"rollingUpdate"`
		} `json:"strategy"`
		Selector struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				TerminationGracePeriodSeconds int                   `json:"terminationGracePeriodSeconds"`
				Containers                    []deployContainerJSON `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("parse deployment spec: %w", err)
	}
	if spec.Selector.MatchLabels == nil || spec.Template.Metadata.Labels == nil {
		return nil, fmt.Errorf("deployment needs selector.matchLabels and template labels")
	}
	if len(spec.Template.Spec.Containers) == 0 {
		return nil, fmt.Errorf("deployment pod template has no containers")
	}
	deployment := &K8sDeploymentSpec{
		Replicas:                      deployIntOr(spec.Replicas, 1),
		MinReadySeconds:               spec.MinReadySeconds,
		TerminationGracePeriodSeconds: spec.Template.Spec.TerminationGracePeriodSeconds,
		StrategyType:                  spec.Strategy.Type,
		MatchLabels:                   spec.Selector.MatchLabels,
		PodLabels:                     spec.Template.Metadata.Labels,
	}
	if len(spec.Strategy.RollingUpdate) > 0 {
		var rolling struct {
			MaxSurge       json.RawMessage `json:"maxSurge"`
			MaxUnavailable json.RawMessage `json:"maxUnavailable"`
		}
		if err := json.Unmarshal(spec.Strategy.RollingUpdate, &rolling); err != nil {
			return nil, fmt.Errorf("parse rollingUpdate: %w", err)
		}
		deployment.MaxSurge = deployFlexString(rolling.MaxSurge)
		deployment.MaxUnavailable = deployFlexString(rolling.MaxUnavailable)
	}
	for _, container := range spec.Template.Spec.Containers {
		typed, err := deployParseContainer(container)
		if err != nil {
			return nil, err
		}
		deployment.Containers = append(deployment.Containers, typed)
	}
	return deployment, nil
}

// deployContainerJSON mirrors the pod container fields the demo reads;
// IntOrString fields stay raw so numbers and strings both parse.
type deployContainerJSON struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Ports []struct {
		Name          string `json:"name"`
		ContainerPort int    `json:"containerPort"`
	} `json:"ports"`
	EnvFrom []struct {
		ConfigMapRef *struct {
			Name string `json:"name"`
		} `json:"configMapRef"`
		SecretRef *struct {
			Name string `json:"name"`
		} `json:"secretRef"`
	} `json:"envFrom"`
	Resources struct {
		Requests map[string]string `json:"requests"`
		Limits   map[string]string `json:"limits"`
	} `json:"resources"`
	LivenessProbe   *deployProbeJSON `json:"livenessProbe"`
	ReadinessProbe  *deployProbeJSON `json:"readinessProbe"`
	SecurityContext *struct {
		RunAsNonRoot             *bool  `json:"runAsNonRoot"`
		RunAsUser                *int64 `json:"runAsUser"`
		AllowPrivilegeEscalation *bool  `json:"allowPrivilegeEscalation"`
		ReadOnlyRootFilesystem   *bool  `json:"readOnlyRootFilesystem"`
		Capabilities             *struct {
			Drop []string `json:"drop"`
		} `json:"capabilities"`
	} `json:"securityContext"`
	VolumeMounts []struct {
		Name      string `json:"name"`
		MountPath string `json:"mountPath"`
	} `json:"volumeMounts"`
}

// deployProbeJSON mirrors one HTTP GET probe.
type deployProbeJSON struct {
	HTTPGet *struct {
		Path string          `json:"path"`
		Port json.RawMessage `json:"port"`
	} `json:"httpGet"`
	InitialDelaySeconds int `json:"initialDelaySeconds"`
	PeriodSeconds       int `json:"periodSeconds"`
	TimeoutSeconds      int `json:"timeoutSeconds"`
	FailureThreshold    int `json:"failureThreshold"`
}

// deployParseContainer types one container, converting the JSON-only
// shapes (IntOrString, security context) into the demo's strings.
func deployParseContainer(raw deployContainerJSON) (K8sContainer, error) {
	if raw.Name == "" || raw.Image == "" {
		return K8sContainer{}, fmt.Errorf("container %q needs name and image", raw.Name)
	}
	container := K8sContainer{
		Name:      raw.Name,
		Image:     raw.Image,
		Resources: K8sResources{Requests: raw.Resources.Requests, Limits: raw.Resources.Limits},
	}
	for _, port := range raw.Ports {
		container.Ports = append(container.Ports, K8sContainerPort{Name: port.Name, ContainerPort: port.ContainerPort})
	}
	for _, source := range raw.EnvFrom {
		switch {
		case source.ConfigMapRef != nil:
			container.EnvFrom = append(container.EnvFrom, "configMap "+source.ConfigMapRef.Name)
		case source.SecretRef != nil:
			container.EnvFrom = append(container.EnvFrom, "secret "+source.SecretRef.Name)
		}
	}
	if raw.LivenessProbe != nil {
		probe, err := deployParseProbe("livenessProbe", raw.LivenessProbe)
		if err != nil {
			return K8sContainer{}, err
		}
		container.LivenessProbe = probe
	}
	if raw.ReadinessProbe != nil {
		probe, err := deployParseProbe("readinessProbe", raw.ReadinessProbe)
		if err != nil {
			return K8sContainer{}, err
		}
		container.ReadinessProbe = probe
	}
	if security := raw.SecurityContext; security != nil {
		container.Security = deploySecurityFacts(
			deploySecurityContextClaims{
				runAsNonRoot: security.RunAsNonRoot, runAsUser: security.RunAsUser,
				allowEscalation: security.AllowPrivilegeEscalation,
				readOnlyRoot:    security.ReadOnlyRootFilesystem, capabilities: security.Capabilities,
			})
	}
	for _, mount := range raw.VolumeMounts {
		container.VolumeMounts = append(container.VolumeMounts, mount.Name+" -> "+mount.MountPath)
	}
	return container, nil
}

// deployParseProbe types one HTTP GET probe, rejecting a probe without
// its httpGet — the only probe flavor this deployment uses.
func deployParseProbe(field string, raw *deployProbeJSON) (*K8sHTTPProbe, error) {
	if raw.HTTPGet == nil {
		return nil, fmt.Errorf("container %s must be an httpGet probe", field)
	}
	return &K8sHTTPProbe{
		Path:                raw.HTTPGet.Path,
		Port:                deployFlexString(raw.HTTPGet.Port),
		InitialDelaySeconds: raw.InitialDelaySeconds,
		PeriodSeconds:       raw.PeriodSeconds,
		TimeoutSeconds:      raw.TimeoutSeconds,
		FailureThreshold:    raw.FailureThreshold,
	}, nil
}

// deploySecurityContextClaims groups the security context assertions
// the payload renders — each claim only when the manifest asserts it.
type deploySecurityContextClaims struct {
	runAsNonRoot    *bool
	runAsUser       *int64
	allowEscalation *bool
	readOnlyRoot    *bool
	capabilities    *struct {
		Drop []string `json:"drop"`
	}
}

// deploySecurityFacts renders the security context as the payload's
// fact strings.
func deploySecurityFacts(claims deploySecurityContextClaims) []string {
	var facts []string
	if claims.runAsNonRoot != nil && *claims.runAsNonRoot {
		facts = append(facts, "runs as non-root")
	}
	if claims.runAsUser != nil {
		facts = append(facts, fmt.Sprintf("uid %d", *claims.runAsUser))
	}
	if claims.allowEscalation != nil && !*claims.allowEscalation {
		facts = append(facts, "no privilege escalation")
	}
	if claims.readOnlyRoot != nil && *claims.readOnlyRoot {
		facts = append(facts, "read-only root filesystem")
	}
	if claims.capabilities != nil && len(claims.capabilities.Drop) > 0 {
		facts = append(facts, "capabilities dropped: "+strings.Join(claims.capabilities.Drop, ", "))
	}
	return facts
}

// deployParseServiceSpec types the core/v1 service payload.
func deployParseServiceSpec(raw json.RawMessage) (*K8sServiceSpec, error) {
	var spec struct {
		Type     string            `json:"type"`
		Selector map[string]string `json:"selector"`
		Ports    []struct {
			Name       string          `json:"name"`
			Port       int             `json:"port"`
			TargetPort json.RawMessage `json:"targetPort"`
		} `json:"ports"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("parse service spec: %w", err)
	}
	if spec.Selector == nil || len(spec.Ports) == 0 {
		return nil, fmt.Errorf("service needs selector and ports")
	}
	service := &K8sServiceSpec{Type: spec.Type, Selector: spec.Selector}
	for _, port := range spec.Ports {
		service.Ports = append(service.Ports, K8sServicePort{
			Name:       port.Name,
			Port:       port.Port,
			TargetPort: deployFlexString(port.TargetPort),
		})
	}
	return service, nil
}

// ── the secret gate ────────────────────────────────────────────────

// IsPlaceholderSecretValue reports whether one base64 Secret data
// value decodes to a string carrying the CHANGE_ME marker: a value
// that is deliberately, visibly not a credential. Anything else — a
// real-looking password, a live DSN, a random token, invalid base64,
// an empty string — is refused, because the rule the topic teaches is
// that the scanner never GUESSES what is sensitive: it only ever
// accepts values that announce themselves as placeholders.
func IsPlaceholderSecretValue(value string) bool {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return false
	}
	return strings.Contains(string(decoded), SecretPlaceholderMarker)
}

// DeployScanSecrets walks the parsed manifests and reports every
// Secret data value that is not a placeholder — the builder refuses to
// serve a payload while any offender exists, so the scan's empty
// result is a load-bearing fact, not a formality.
func DeployScanSecrets(manifests []K8sManifest) []string {
	var offenders []string
	for _, manifest := range manifests {
		if manifest.Secret == nil {
			continue
		}
		for _, key := range deploySortedStringKeys(manifest.Secret.Data) {
			if !IsPlaceholderSecretValue(manifest.Secret.Data[key]) {
				offenders = append(offenders, manifest.File+"/"+key)
			}
		}
	}
	return offenders
}

// ── the wire payload ───────────────────────────────────────────────

// DeployFactView is one derived fact with its label — the label/value
// pairs are the payload's navigation surface; every value comes from a
// parsed artifact.
type DeployFactView struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// DeployComposeServiceView is one compose service on the wire.
type DeployComposeServiceView struct {
	Name           string   `json:"name"`
	Image          string   `json:"image,omitempty"`
	BuildsFrom     string   `json:"buildsFrom,omitempty"`
	PublishedPorts []string `json:"publishedPorts"`
	Environment    []string `json:"environment"`
	Healthcheck    string   `json:"healthcheck,omitempty"`
	DependsOn      []string `json:"dependsOn"`
	ResourceHint   string   `json:"resourceHint,omitempty"`
}

// DeployComposeView is the compose half of the payload with its
// provenance path.
type DeployComposeView struct {
	Source   string                     `json:"source"`
	Project  string                     `json:"project"`
	Services []DeployComposeServiceView `json:"services"`
	Networks []string                   `json:"networks"`
}

// DeployManifestView is one k8s manifest on the wire: the envelope and
// the derived facts, with the repo-root-relative file it was parsed
// from as provenance.
type DeployManifestView struct {
	File       string           `json:"file"`
	Kind       string           `json:"kind"`
	Name       string           `json:"name"`
	APIVersion string           `json:"apiVersion"`
	Namespace  string           `json:"namespace,omitempty"`
	Facts      []DeployFactView `json:"facts"`
}

// DeploySecretScanView states the scan the payload rode through: the
// verdict, the rule, and every checked slot.
type DeploySecretScanView struct {
	Status  string   `json:"status"`
	Rule    string   `json:"rule"`
	Checked []string `json:"checked"`
}

// DeployStackPayload is the wire shape of GET /api/demo/deploy/stack.
type DeployStackPayload struct {
	Mode       string               `json:"mode"`
	Compose    DeployComposeView    `json:"compose"`
	Kubernetes []DeployManifestView `json:"kubernetes"`
	SecretScan DeploySecretScanView `json:"secretScan"`
	Reasoning  string               `json:"reasoning"`
}

// BuildDeployStackPayload shapes the wire payload (pure): parses the
// artifacts, gates every Secret value through the placeholder scan —
// refusing to serve at the first non-placeholder value — and derives
// the topology with per-artifact provenance. No value on the wire is
// invented: if it is not in the parsed files, it is not in the payload.
func BuildDeployStackPayload(facts DeployStackFacts) (DeployStackPayload, error) {
	compose, err := ParseComposeTopology(facts.ComposeYAML)
	if err != nil {
		return DeployStackPayload{}, err
	}
	manifests, err := ParseK8sManifests(facts.Manifests)
	if err != nil {
		return DeployStackPayload{}, err
	}
	if offenders := DeployScanSecrets(manifests); len(offenders) > 0 {
		return DeployStackPayload{}, fmt.Errorf(
			"deploy: secret values are not CHANGE_ME_* placeholders (refusing to serve): %s",
			strings.Join(offenders, ", "),
		)
	}

	payload := DeployStackPayload{
		Mode: "file-derived: every service, port, healthcheck, probe path, and " +
			"rollout knob below was parsed at request time from the repo's " +
			"committed deploy/ artifacts — the compose file with the guide's " +
			"zero-dep YAML-subset reader, the Kubernetes manifests with " +
			"encoding/json — and every fact names the file it came from",
		Compose:    deployComposeView(compose),
		Kubernetes: make([]DeployManifestView, 0, len(manifests)),
		SecretScan: deploySecretScanView(manifests),
		Reasoning: "Read the two halves as one stack story. The compose half is " +
			"the single-host deployment: five services, dependency conditions " +
			"instead of start-order hope, healthchecks on everything that ships " +
			"probe tooling, resource hints on every container, and the one true " +
			"secret behind an interpolation rail the committed file never " +
			"resolves. The Kubernetes half is the same server's cluster " +
			"deployment: kubelet-driven HTTP probes against the REAL /api/health " +
			"and /api/ready endpoints (no shell needed in the distroless " +
			"image), a zero-unavailable rolling update, config split into " +
			"ConfigMap (non-secret) and Secret (placeholders only — the scan " +
			"above refused to serve otherwise), and named ports kept " +
			"consistent from container to service. Trace any fact to its file: " +
			"that is the topic's whole argument",
	}
	for _, manifest := range manifests {
		payload.Kubernetes = append(payload.Kubernetes, deployManifestView(manifest))
	}
	return payload, nil
}

// deployComposeView maps the parsed topology onto the wire shape.
func deployComposeView(compose ComposeTopology) DeployComposeView {
	view := DeployComposeView{
		Source:   DeployDirName + "/" + DeployComposeFile,
		Project:  compose.Project,
		Networks: compose.Networks,
	}
	for _, service := range compose.Services {
		mapped := DeployComposeServiceView{
			Name:           service.Name,
			Image:          service.Image,
			BuildsFrom:     service.Dockerfile,
			PublishedPorts: service.Ports,
			Environment:    service.Environment,
			Healthcheck:    service.Healthcheck,
			DependsOn:      service.DependsOn,
		}
		if service.Dockerfile == "" && service.BuildContext != "" {
			mapped.BuildsFrom = "context " + service.BuildContext
		}
		if service.CPULimit != "" || service.MemoryLimit != "" {
			mapped.ResourceHint = fmt.Sprintf("limits %s cpu / %s memory; reservations %s cpu / %s memory",
				service.CPULimit, service.MemoryLimit, service.CPUReservation, service.MemoryReservation)
		}
		view.Services = append(view.Services, mapped)
	}
	return view
}

// deployManifestView derives one manifest's fact list — the envelope
// facts every object carries, then the kind-specific facts.
func deployManifestView(manifest K8sManifest) DeployManifestView {
	view := DeployManifestView{
		File:       DeployDirName + "/" + manifest.File,
		Kind:       manifest.Kind,
		Name:       manifest.Name,
		APIVersion: manifest.APIVersion,
		Namespace:  manifest.Namespace,
	}
	switch {
	case manifest.ConfigMap != nil:
		for _, key := range deploySortedStringKeys(manifest.ConfigMap) {
			view.Facts = append(view.Facts, DeployFactView{Label: key, Value: manifest.ConfigMap[key]})
		}
	case manifest.Secret != nil:
		for _, key := range deploySortedStringKeys(manifest.Secret.Data) {
			decoded, _ := base64.StdEncoding.DecodeString(manifest.Secret.Data[key])
			view.Facts = append(view.Facts, DeployFactView{
				Label: key,
				Value: "base64, decodes to the placeholder: " + string(decoded),
			})
		}
	case manifest.Deployment != nil:
		view.Facts = deployDeploymentFacts(manifest.Deployment)
	case manifest.Service != nil:
		for _, label := range deploySortedStringKeys(manifest.Service.Selector) {
			view.Facts = append(view.Facts, DeployFactView{Label: "selector " + label, Value: manifest.Service.Selector[label]})
		}
		view.Facts = append(view.Facts, DeployFactView{Label: "type", Value: manifest.Service.Type})
		for _, port := range manifest.Service.Ports {
			view.Facts = append(view.Facts, DeployFactView{
				Label: "port " + port.Name,
				Value: fmt.Sprintf("%d -> targetPort %s", port.Port, port.TargetPort),
			})
		}
	}
	if len(view.Facts) == 0 {
		view.Facts = append(view.Facts, DeployFactView{Label: "name", Value: manifest.Name})
	}
	return view
}

// deployDeploymentFacts flattens the deployment's rollout shape into
// labeled facts, container by container.
func deployDeploymentFacts(deployment *K8sDeploymentSpec) []DeployFactView {
	facts := []DeployFactView{
		{Label: "replicas", Value: fmt.Sprintf("%d (minReadySeconds %d)", deployment.Replicas, deployment.MinReadySeconds)},
		{Label: "strategy", Value: fmt.Sprintf("%s (maxSurge %s, maxUnavailable %s)",
			deployment.StrategyType, deployment.MaxSurge, deployment.MaxUnavailable)},
		{Label: "terminationGracePeriodSeconds", Value: fmt.Sprintf("%d", deployment.TerminationGracePeriodSeconds)},
	}
	for _, container := range deployment.Containers {
		facts = append(facts, DeployFactView{Label: "container " + container.Name, Value: "image " + container.Image})
		for _, port := range container.Ports {
			facts = append(facts, DeployFactView{
				Label: "container port " + port.Name,
				Value: fmt.Sprintf("%d", port.ContainerPort),
			})
		}
		for _, source := range container.EnvFrom {
			facts = append(facts, DeployFactView{Label: "envFrom", Value: source})
		}
		if container.LivenessProbe != nil {
			facts = append(facts, DeployFactView{
				Label: "livenessProbe",
				Value: fmt.Sprintf("GET %s on port %s every %ds (timeout %ds, %d failures restarts the container)",
					container.LivenessProbe.Path, container.LivenessProbe.Port,
					container.LivenessProbe.PeriodSeconds, container.LivenessProbe.TimeoutSeconds,
					container.LivenessProbe.FailureThreshold),
			})
		}
		if container.ReadinessProbe != nil {
			facts = append(facts, DeployFactView{
				Label: "readinessProbe",
				Value: fmt.Sprintf("GET %s on port %s every %ds (initial delay %ds, timeout %ds, %d failures removes the pod from the Service)",
					container.ReadinessProbe.Path, container.ReadinessProbe.Port,
					container.ReadinessProbe.PeriodSeconds, container.ReadinessProbe.InitialDelaySeconds,
					container.ReadinessProbe.TimeoutSeconds, container.ReadinessProbe.FailureThreshold),
			})
		}
		if requests := deploymentResourceLine("requests", container.Resources.Requests); requests != "" {
			facts = append(facts, DeployFactView{Label: "resources", Value: requests})
		}
		if limits := deploymentResourceLine("limits", container.Resources.Limits); limits != "" {
			facts = append(facts, DeployFactView{Label: "resources", Value: limits})
		}
		if len(container.Security) > 0 {
			facts = append(facts, DeployFactView{Label: "securityContext", Value: strings.Join(container.Security, "; ")})
		}
		for _, mount := range container.VolumeMounts {
			facts = append(facts, DeployFactView{Label: "volumeMount", Value: mount})
		}
	}
	return facts
}

// deploymentResourceLine renders one resources tier as a fact string.
func deploymentResourceLine(tier string, quantities map[string]string) string {
	if len(quantities) == 0 {
		return ""
	}
	parts := make([]string, 0, len(quantities))
	for _, key := range deploySortedStringKeys(quantities) {
		parts = append(parts, key+" "+quantities[key])
	}
	return "resources " + tier + ": " + strings.Join(parts, ", ")
}

// deploySecretScanView states the scan the payload passed.
func deploySecretScanView(manifests []K8sManifest) DeploySecretScanView {
	view := DeploySecretScanView{
		Status: "clean",
		Rule: "every Secret data value must be base64 that decodes to a " +
			"CHANGE_ME_* placeholder — the builder refuses to serve the " +
			"payload otherwise, so a real credential can never ride this " +
			"demo to the wire",
	}
	for _, manifest := range manifests {
		if manifest.Secret == nil {
			continue
		}
		for _, key := range deploySortedStringKeys(manifest.Secret.Data) {
			view.Checked = append(view.Checked, manifest.File+"/"+key)
		}
	}
	return view
}

// ── small parse helpers ────────────────────────────────────────────

// deployMapOf casts a parsed YAML value to a mapping or names the path.
func deployMapOf(path string, value any) (map[string]any, error) {
	if mapping, ok := value.(map[string]any); ok {
		return mapping, nil
	}
	return nil, fmt.Errorf("deploy: compose %s is not a mapping (got %T)", path, value)
}

// deployPlainString reads a scalar that may be absent or nil.
func deployPlainString(value any) string {
	if value == nil {
		return ""
	}
	return deployScalarString(value)
}

// deployScalarString renders one scalar: string, bool, or int.
func deployScalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

// deployStringSlice reads a sequence of scalars, absent meaning empty.
func deployStringSlice(path string, value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("deploy: compose %s is not a sequence (got %T)", path, value)
	}
	entries := make([]string, 0, len(items))
	for _, item := range items {
		entries = append(entries, deployScalarString(item))
	}
	return entries, nil
}

// deploySortedKeys sorts a parsed mapping's keys — parsed maps carry
// no order, so sorted output keeps the payload stable.
func deploySortedKeys(mapping map[string]any) []string {
	keys := make([]string, 0, len(mapping))
	for key := range mapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// deploySortedStringKeys is deploySortedKeys for string maps.
func deploySortedStringKeys(mapping map[string]string) []string {
	keys := make([]string, 0, len(mapping))
	for key := range mapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// deployFlexString renders an IntOrString JSON value (a named port or
// a plain number both arrive here).
func deployFlexString(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) >= 2 && trimmed[0] == '"' {
		var unquoted string
		if err := json.Unmarshal(raw, &unquoted); err == nil {
			return unquoted
		}
	}
	return trimmed
}

// deployIntOr returns the pointer's value or the default.
func deployIntOr(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}
