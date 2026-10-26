package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The Phase 6 testcontainers wiring proof, and the template the real-PG
// tests (topic 18 sqlc-gorm) copy: a test inside the dev container drives
// the HOST Docker daemon through the mounted socket, spawns a throwaway
// postgres:17 SIBLING container, reaches it over pgx through the port the
// host daemon published for it, and terminates it in cleanup (Ryuk reaps
// anything a crashed run leaves behind — it stays enabled on purpose).
// On machines without the socket wiring the test skips, so the suite
// never hard-depends on it.

// dockerSocket is where compose mounts the host daemon into the dev
// container (.docker/docker-compose.yml, dev service volumes).
const dockerSocket = "/var/run/docker.sock"

// testContainerStartup bounds image create + PostgreSQL's own boot. The
// postgres:17 image is already on the host (it IS the stack's engine), so
// this is generous rather than tight.
const testContainerStartup = 90 * time.Second

func TestTestcontainersSmoke(t *testing.T) {
	if _, err := os.Stat(dockerSocket); err != nil {
		t.Skipf("docker socket unavailable at %s: testcontainers wiring not present on this machine", dockerSocket)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	req := testcontainers.ContainerRequest{
		// The guide's pinned engine image — same as the compose stack's
		// postgres service, so no extra pull on the host daemon.
		Image:        "postgres:17",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "guide",
			"POSTGRES_DB":       "guide",
		},
		// postgres logs "ready to accept connections" twice — once for the
		// init phase, once for the real server after its restart. Only the
		// second one answers queries.
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(testContainerStartup),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		terminateCtx, terminateCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer terminateCancel()
		if err := container.Terminate(terminateCtx); err != nil {
			t.Logf("terminate container: %v (Ryuk reaps it regardless)", err)
		}
	})

	// container.Host honors TESTCONTAINERS_HOST_OVERRIDE: the spawned
	// sibling is reachable through its PUBLISHED port on the Docker
	// Desktop host alias, not through the compose bridge gateway.
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("resolve container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("resolve mapped port: %v", err)
	}
	dsn := fmt.Sprintf("postgres://postgres:guide@%s:%s/guide?sslmode=disable", host, port.Port())

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect spawned postgres: %v", err)
	}
	defer conn.Close(context.Background())

	var got int
	if err := conn.QueryRow(ctx, "SELECT 1").Scan(&got); err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	if got != 1 {
		t.Fatalf("SELECT 1 = %d, want 1", got)
	}
}
