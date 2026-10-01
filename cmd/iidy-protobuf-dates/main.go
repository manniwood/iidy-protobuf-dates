package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/manniwood/iidy-protobuf-dates/data"
	"github.com/manniwood/iidy-protobuf-dates/migrations"
	pb "github.com/manniwood/iidy-protobuf-dates/pb/iidy"
	"github.com/manniwood/iidy-protobuf-dates/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

const defaultPort string = "8080"

func main() {
	port := os.Getenv("IIDY_PORT")
	if port == "" {
		port = defaultPort
	}

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %v: %v", port, err)
	}

	// Configure server options for production use
	serverOptions := []grpc.ServerOption{
		// Set maximum message sizes
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10MB

		// Configure keepalive settings to detect dead connections
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  5 * time.Minute,
			Timeout:               20 * time.Second,
		}),

		// Enforce keepalive policies on clients
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// Create the gRPC server with our options
	grpcServer := grpc.NewServer(serverOptions...)

	ctx, cFunc := context.WithCancel(context.Background())
	defer cFunc()

	poolURL := os.Getenv("IIDY_PG_POOL_URL")
	if poolURL == "" {
		poolURL = data.DefaultPgPoolURL
	}

	migrationURL := os.Getenv("IIDY_PG_MIGRATION_URL")
	if migrationURL == "" {
		migrationURL = data.DefaultPgMigrationURL
	}

	migrationConn, err := data.CreatePGXConnForMigration(ctx, migrationURL)
	if err != nil {
		log.Fatalf("Could not create connection for migration: %v\n", err)
	}
	defer migrationConn.Close(ctx)
	err = data.MigrateDB(ctx, migrationConn, migrations.Migrations, data.TernMigrationTable)
	if err != nil {
		log.Fatalf("Could not migrate db: %v\n", err)
	}
	// Don't need this single connection anymore, so close it.
	migrationConn.Close(ctx)

	data.PgxPool, err = data.CreatePGXPool(ctx, poolURL)
	if err != nil {
		log.Fatalf("Could not create connection pool: %v\n", err)
	}
	defer data.PgxPool.Close()

	// Create and register our user service
	iidyService := service.NewIIDYService(data.PgxPool)
	pb.RegisterIIDYServiceServer(grpcServer, iidyService)

	// Enable server reflection for debugging with tools like grpcurl
	reflection.Register(grpcServer)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Received shutdown signal, initiating graceful shutdown...")

		// Create a context with timeout for graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Stop accepting new connections and wait for existing ones
		stopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(stopped)
		}()

		select {
		case <-stopped:
			log.Println("Server stopped gracefully")
		case <-ctx.Done():
			log.Println("Shutdown timeout, forcing stop")
			grpcServer.Stop()
		}
	}()

	log.Printf("gRPC server listening on %s", listener.Addr().String())

	// Start serving requests
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
