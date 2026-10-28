package demos

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Topic 37 deployment — the stack viewer's belt. The tests pin the
// demo's whole honesty: the REAL artifacts under deploy/ parse into
// the typed shapes (compose services, dependency conditions, k8s probe
// paths wired to the REAL /api/health + /api/ready endpoints, label/
// selector consistency, ports coherent across configmap/deployment/
// service), the secret scanner table-tests its rule and the builder
// REFUSES a non-placeholder secret, and the served payload carries
// per-artifact provenance with no invented values. Everything here
// reads the committed files through the same LocateDeployDir walk the
// composition root uses — an artifact moved or renamed fails these
// tests, not the demo.

// deployStackRealFacts loads the repo's committed deployment artifacts
// exactly the way the handler does: the compose file's bytes plus every
// k8s manifest, names relative to the deploy directory.
func deployStackRealFacts(t *testing.T) DeployStackFacts {
	t.Helper()
	dir := LocateDeployDir(".")
	if dir == "" {
		t.Fatal("LocateDeployDir could not find deploy/ from the test working directory — the artifacts must sit beside the module")
	}
	compose, err := os.ReadFile(filepath.Join(dir, DeployComposeFile))
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, DeployK8sDir, "*.json"))
	if err != nil {
		t.Fatalf("glob k8s manifests: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no k8s manifests found under deploy/k8s/")
	}
	facts := DeployStackFacts{ComposeYAML: compose}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read manifest %s: %v", path, err)
		}
		facts.Manifests = append(facts.Manifests, DeployManifestFile{
			Name: DeployK8sDir + "/" + filepath.Base(path),
			JSON: raw,
		})
	}
	return facts
}

// deployStackServiceByName finds one parsed compose service.
func deployStackServiceByName(t *testing.T, topology ComposeTopology, name string) ComposeService {
	t.Helper()
	for _, service := range topology.Services {
		if service.Name == name {
			return service
		}
	}
	t.Fatalf("compose service %q missing from %v", name, topology.Services)
	return ComposeService{}
}

// deployStackManifestByKind finds one parsed k8s manifest by kind.
func deployStackManifestByKind(t *testing.T, manifests []K8sManifest, kind string) K8sManifest {
	t.Helper()
	for _, manifest := range manifests {
		if manifest.Kind == kind {
			return manifest
		}
	}
	t.Fatalf("k8s manifest kind %q missing", kind)
	return K8sManifest{}
}

func TestDeployStackLocatesTheRealArtifactsAndNothingElse(t *testing.T) {
	if dir := LocateDeployDir("."); dir == "" {
		t.Error("LocateDeployDir must find deploy/docker-compose.prod.yml walking up from the package dir")
	}
	if dir := LocateDeployDir(t.TempDir()); dir != "" {
		t.Errorf("LocateDeployDir outside the repo = %q, want empty — no ancestor carries the artifacts", dir)
	}
}

func TestDeployStackParsesTheRealComposeTopology(t *testing.T) {
	topology, err := ParseComposeTopology(deployStackRealFacts(t).ComposeYAML)
	if err != nil {
		t.Fatalf("parse the real compose file: %v", err)
	}
	if topology.Project != "go_developers_guide_deploy" {
		t.Errorf("project = %q, want go_developers_guide_deploy", topology.Project)
	}
	if strings.Join(topology.Networks, ",") != "app_internal" {
		t.Errorf("networks = %v, want the one internal network", topology.Networks)
	}
	if len(topology.Services) != 5 {
		t.Fatalf("services = %d, want 5 (server, client, postgres, redis, mongo)", len(topology.Services))
	}
	got := make([]string, 0, 5)
	for _, service := range topology.Services {
		got = append(got, service.Name)
	}
	if strings.Join(got, ",") != "client,mongo,postgres,redis,server" {
		t.Errorf("services (sorted) = %v, want the five stack members", got)
	}

	server := deployStackServiceByName(t, topology, "server")
	if strings.Join(server.Ports, ",") != "8100:8100" {
		t.Errorf("server ports = %v, want the single published API port", server.Ports)
	}
	if server.Dockerfile != "./.docker/server/Dockerfile.prod" {
		t.Errorf("server builds from %q, want the real prod Dockerfile", server.Dockerfile)
	}
	if server.Healthcheck != "" {
		t.Errorf("server healthcheck = %q, want none — the distroless image has nothing to exec; probing is the platform's job", server.Healthcheck)
	}
	if len(server.DependsOn) != 3 {
		t.Fatalf("server depends_on = %v, want all three datastores", server.DependsOn)
	}
	for _, dep := range server.DependsOn {
		if !strings.HasSuffix(dep, " (service_healthy)") {
			t.Errorf("depends_on entry %q must carry the service_healthy condition", dep)
		}
	}
	railOnWire := false
	for _, entry := range server.Environment {
		if strings.HasPrefix(entry, "GDG_DATABASE_URL=postgres://guide:${POSTGRES_PASSWORD:?") {
			railOnWire = true
		}
	}
	if !railOnWire {
		t.Errorf("server environment must carry the DSN interpolation rail verbatim, got %v", server.Environment)
	}
	if server.CPULimit != "1.0" || server.MemoryLimit != "256M" {
		t.Errorf("server resource limits = %q/%q, want the authored hint", server.CPULimit, server.MemoryLimit)
	}

	client := deployStackServiceByName(t, topology, "client")
	if !strings.Contains(client.Healthcheck, "/api/health") {
		t.Errorf("client healthcheck must prove the proxied API path, got %q", client.Healthcheck)
	}

	// The family prod stance: datastores never publish host ports.
	for _, name := range []string{"postgres", "redis", "mongo"} {
		if service := deployStackServiceByName(t, topology, name); len(service.Ports) != 0 {
			t.Errorf("%s publishes %v — prod datastores stay inside the internal network", name, service.Ports)
		}
	}
	postgres := deployStackServiceByName(t, topology, "postgres")
	if !strings.Contains(postgres.Healthcheck, "pg_isready -U guide -d guide") {
		t.Errorf("postgres healthcheck = %q, want pg_isready against the guide role", postgres.Healthcheck)
	}
}

func TestDeployStackParsesTheRealK8sManifests(t *testing.T) {
	manifests, err := ParseK8sManifests(deployStackRealFacts(t).Manifests)
	if err != nil {
		t.Fatalf("parse the real manifests: %v", err)
	}
	kinds := make(map[string]bool)
	for _, manifest := range manifests {
		kinds[manifest.Kind] = true
	}
	for _, want := range []string{"Namespace", "ConfigMap", "Secret", "Deployment", "Service"} {
		if !kinds[want] {
			t.Errorf("manifest set lacks a %s (got kinds %v)", want, kinds)
		}
	}

	namespace := deployStackManifestByKind(t, manifests, "Namespace").Name

	deployment := deployStackManifestByKind(t, manifests, "Deployment").Deployment
	if deployment.Replicas != 2 {
		t.Errorf("replicas = %d, want 2", deployment.Replicas)
	}
	if deployment.StrategyType != "RollingUpdate" || deployment.MaxSurge != "1" || deployment.MaxUnavailable != "0" {
		t.Errorf("rollout knobs = %s/%s/%s, want RollingUpdate surge 1 unavailable 0",
			deployment.StrategyType, deployment.MaxSurge, deployment.MaxUnavailable)
	}
	if deployment.TerminationGracePeriodSeconds != 30 {
		t.Errorf("terminationGracePeriodSeconds = %d, want the 30s drain window", deployment.TerminationGracePeriodSeconds)
	}
	if len(deployment.Containers) != 1 {
		t.Fatalf("containers = %d, want the single guide container", len(deployment.Containers))
	}
	container := deployment.Containers[0]

	// The probe contract: kubelet GETs the REAL endpoints the server
	// serves (topic 33's /api/health liveness and /api/ready readiness),
	// on the named http port.
	if container.LivenessProbe == nil || container.LivenessProbe.Path != "/api/health" || container.LivenessProbe.Port != "http" {
		t.Errorf("livenessProbe = %+v, want GET /api/health on the named http port", container.LivenessProbe)
	}
	if container.ReadinessProbe == nil || container.ReadinessProbe.Path != "/api/ready" || container.ReadinessProbe.Port != "http" {
		t.Errorf("readinessProbe = %+v, want GET /api/ready on the named http port", container.ReadinessProbe)
	}

	portsByName := make(map[string]int)
	for _, port := range container.Ports {
		portsByName[port.Name] = port.ContainerPort
	}
	if portsByName["http"] != 8100 || portsByName["grpc"] != 8102 {
		t.Errorf("container ports = %v, want http 8100 and grpc 8102", portsByName)
	}
	envFrom := strings.Join(container.EnvFrom, ",")
	if !strings.Contains(envFrom, "configMap guide-config") || !strings.Contains(envFrom, "secret guide-secret") {
		t.Errorf("envFrom = %q, want both the configmap and the secret", envFrom)
	}
	if container.Resources.Requests["cpu"] != "100m" || container.Resources.Limits["memory"] != "256Mi" {
		t.Errorf("resources = %+v, want the authored requests/limits", container.Resources)
	}
	security := strings.Join(container.Security, "; ")
	for _, want := range []string{"non-root", "uid 65532", "read-only root filesystem", "ALL"} {
		if !strings.Contains(security, want) {
			t.Errorf("securityContext facts %q lack %q", security, want)
		}
	}
	if len(container.VolumeMounts) != 1 || !strings.HasPrefix(container.VolumeMounts[0], "tmp -> /tmp") {
		t.Errorf("volumeMounts = %v, want the writable tmp emptyDir the read-only rootfs requires", container.VolumeMounts)
	}

	service := deployStackManifestByKind(t, manifests, "Service").Service
	for label, value := range deployment.MatchLabels {
		if service.Selector[label] != value {
			t.Errorf("service selector %s = %q, deployment matchLabels says %q", label, service.Selector[label], value)
		}
		if deployment.PodLabels[label] != value {
			t.Errorf("pod labels %s = %q, deployment matchLabels says %q", label, deployment.PodLabels[label], value)
		}
	}
	for _, port := range service.Ports {
		if _, ok := portsByName[port.TargetPort]; !ok {
			t.Errorf("service port %s targets %q, no such container port name", port.Name, port.TargetPort)
		}
		if port.Port != portsByName[port.TargetPort] {
			t.Errorf("service port %s = %d but its target container port is %d — the numbers must agree",
				port.Name, port.Port, portsByName[port.TargetPort])
		}
	}

	// Every namespaced manifest lands in the namespace the set declares.
	for _, manifest := range manifests {
		if manifest.Kind == "Namespace" {
			continue
		}
		if manifest.Namespace != namespace {
			t.Errorf("%s/%s namespace = %q, want %q", manifest.Kind, manifest.Name, manifest.Namespace, namespace)
		}
	}

	configMap := deployStackManifestByKind(t, manifests, "ConfigMap").ConfigMap
	if configMap["GDG_PORT"] != "8100" || portsByName["http"] != 8100 {
		t.Errorf("configmap GDG_PORT = %q while the container listens on %d — the config and the port must agree",
			configMap["GDG_PORT"], portsByName["http"])
	}
	if _, carriesSecret := configMap["GDG_DATABASE_URL"]; carriesSecret {
		t.Error("configmap must NOT carry GDG_DATABASE_URL — the one credential-bearing value lives in the secret")
	}
}

func TestDeployStackSecretScannerHoldsTheLine(t *testing.T) {
	// The scanner's rule, table-tested: only values that decode to a
	// CHANGE_ME_* marker pass; everything a tired human might commit is
	// refused, and so is everything malformed or empty.
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{"the committed placeholder DSN", base64.StdEncoding.EncodeToString([]byte("postgres://guide:CHANGE_ME_DB_PASSWORD@postgres:5432/guide?sslmode=disable")), true},
		{"a bare marker", base64.StdEncoding.EncodeToString([]byte("CHANGE_ME_API_TOKEN")), true},
		{"a real-looking password", base64.StdEncoding.EncodeToString([]byte("hunter2-SuperSecret!")), false},
		{"a live DSN with credentials", base64.StdEncoding.EncodeToString([]byte("postgres://guide:hunter2@postgres:5432/guide")), false},
		{"a random token", base64.StdEncoding.EncodeToString([]byte("T9xLm2Qv7RaN4eU1Zs8Yw3Ko6Pb5Hj0Df4Cg2")), false},
		{"invalid base64", "not-base64!!", false},
		{"empty", "", false},
	}
	for _, testCase := range cases {
		if got := IsPlaceholderSecretValue(testCase.value); got != testCase.want {
			t.Errorf("%s: IsPlaceholderSecretValue = %v, want %v", testCase.name, got, testCase.want)
		}
	}

	// The real manifest set is clean, and a doctored copy is caught by
	// exactly the doctored key.
	manifests, err := ParseK8sManifests(deployStackRealFacts(t).Manifests)
	if err != nil {
		t.Fatalf("parse real manifests: %v", err)
	}
	if offenders := DeployScanSecrets(manifests); len(offenders) != 0 {
		t.Fatalf("the committed secret manifest must scan clean, got %v", offenders)
	}
	doctored := make([]K8sManifest, len(manifests))
	copy(doctored, manifests)
	for i := range doctored {
		if doctored[i].Secret == nil {
			continue
		}
		leaked := make(map[string]string, len(doctored[i].Secret.Data))
		for key, value := range doctored[i].Secret.Data {
			leaked[key] = value
		}
		leaked["GDG_DATABASE_URL"] = base64.StdEncoding.EncodeToString([]byte("postgres://guide:hunter2@postgres:5432/guide"))
		doctored[i].Secret = &K8sSecretSpec{Type: doctored[i].Secret.Type, Data: leaked}
	}
	if offenders := DeployScanSecrets(doctored); len(offenders) != 1 || !strings.Contains(offenders[0], "GDG_DATABASE_URL") {
		t.Errorf("doctored secret scan = %v, want exactly the GDG_DATABASE_URL offender", offenders)
	}
}

func TestDeployStackBuilderServesTheRealPayload(t *testing.T) {
	payload, err := BuildDeployStackPayload(deployStackRealFacts(t))
	if err != nil {
		t.Fatalf("build the real payload: %v", err)
	}
	if payload.Compose.Source != "deploy/docker-compose.prod.yml" {
		t.Errorf("compose provenance = %q", payload.Compose.Source)
	}
	if len(payload.Compose.Services) != 5 {
		t.Errorf("compose services on the wire = %d, want 5", len(payload.Compose.Services))
	}
	if len(payload.Kubernetes) != 5 {
		t.Errorf("manifest views = %d, want 5", len(payload.Kubernetes))
	}
	files := make([]string, 0, len(payload.Kubernetes))
	for _, view := range payload.Kubernetes {
		files = append(files, view.File)
		if len(view.Facts) == 0 {
			t.Errorf("manifest view %s carries no facts", view.File)
		}
	}
	joined := strings.Join(files, ",")
	for _, want := range []string{"deploy/k8s/deployment.json", "deploy/k8s/service.json", "deploy/k8s/secret.json"} {
		if !strings.Contains(joined, want) {
			t.Errorf("manifest provenance %v lacks %s", files, want)
		}
	}

	// The probe contract rides the payload: the REAL endpoints appear
	// as deployment facts, and the secret scan states its verdict.
	deploymentFacts := ""
	for _, view := range payload.Kubernetes {
		if strings.HasSuffix(view.File, "deployment.json") {
			for _, fact := range view.Facts {
				deploymentFacts += fact.Value + "\n"
			}
		}
	}
	for _, want := range []string{"/api/health", "/api/ready", "maxUnavailable 0"} {
		if !strings.Contains(deploymentFacts, want) {
			t.Errorf("deployment facts lack %q:\n%s", want, deploymentFacts)
		}
	}
	if payload.SecretScan.Status != "clean" {
		t.Errorf("secret scan status = %q, want clean", payload.SecretScan.Status)
	}
	if strings.Join(payload.SecretScan.Checked, ",") != "k8s/secret.json/GDG_DATABASE_URL" {
		t.Errorf("secret scan checked = %v", payload.SecretScan.Checked)
	}

	// The whole payload marshals, carries the decoded placeholder (safe
	// by construction — it announced itself), and stays deterministic.
	once, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if !strings.Contains(string(once), "CHANGE_ME_DB_PASSWORD") {
		t.Error("the secret view must show the decoded PLACEHOLDER for traceability")
	}
	if strings.Contains(string(once), "hunter2") {
		t.Error("no test credential may leak into the payload")
	}
	again, err := BuildDeployStackPayload(deployStackRealFacts(t))
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	twice, err := json.Marshal(again)
	if err != nil {
		t.Fatalf("marshal rebuild: %v", err)
	}
	if string(once) != string(twice) {
		t.Error("two builds of the same artifacts must be byte-identical — the payload is derived, not sampled")
	}
}

func TestDeployStackBuilderRefusesARealSecret(t *testing.T) {
	facts := deployStackRealFacts(t)
	for i, manifest := range facts.Manifests {
		if !strings.Contains(manifest.Name, "secret.json") {
			continue
		}
		doctored := []byte(strings.Replace(string(manifest.JSON),
			"cG9zdGdyZXM6Ly9ndWlkZTpDSEFOR0VfTUVfREJfUEFTU1dPUkRAcG9zdGdyZXM6NTQzMi9ndWlkZT9zc2xtb2RlPWRpc2FibGU=",
			base64.StdEncoding.EncodeToString([]byte("postgres://guide:hunter2@postgres:5432/guide")), 1))
		facts.Manifests[i].JSON = doctored
	}
	_, err := BuildDeployStackPayload(facts)
	if err == nil {
		t.Fatal("the builder must refuse to serve a secret value that is not a CHANGE_ME_* placeholder")
	}
	if !strings.Contains(err.Error(), "k8s/secret.json/GDG_DATABASE_URL") {
		t.Errorf("refusal must name the offending slot, got: %v", err)
	}
}

func TestDeployStackParsersRejectBrokenInput(t *testing.T) {
	if _, err := ParseComposeTopology([]byte("services: [unclosed")); err == nil {
		t.Error("malformed YAML must fail the compose parser, not parse into something wrong")
	}
	if _, err := ParseK8sManifests([]DeployManifestFile{{Name: "k8s/broken.json", JSON: []byte("{not json")}}); err == nil {
		t.Error("malformed JSON must fail the manifest parser")
	}
	if _, err := ParseK8sManifests([]DeployManifestFile{{Name: "k8s/no-kind.json", JSON: []byte(`{"metadata":{"name":"x"}}`)}}); err == nil {
		t.Error("a manifest without kind must be refused")
	}
	if _, err := ParseK8sManifests([]DeployManifestFile{{Name: "k8s/empty-deploy.json", JSON: []byte(`{"kind":"Deployment","metadata":{"name":"x"},"spec":{"selector":{}},"template":{"metadata":{"labels":{}},"spec":{"containers":[]}}}`)}}); err == nil {
		t.Error("a deployment without containers must be refused")
	}
	if _, err := BuildDeployStackPayload(DeployStackFacts{ComposeYAML: []byte("services: {}\n")}); err == nil {
		t.Error("an artifact set without the k8s manifests must fail the builder (deployMapOf names the missing path)")
	}
}
