package iidy

// These tests do ***NOT RUN BY DEFAULT*** when you type `go test ./...`
//
// WARNING: Running these integration tests ***WILL DESTROY YOUR DATABASE***.
//
// To run the integration test for data, you need 1) a locally-running
// PostgreSQL cluster, and 2) to set the following two env vars:
//
//	export INTEGTEST_DESTROY_DB_URL=postgres://postgres:postgres@localhost:5432/postgres
//	export INTEGTEST_DESTROY_DB_I_MEAN_IT=true
//
// To run the integration test for the server binary, you need to have 1) a locally-running
// PostgreSQL cluster, 2) the server running and pointed at port 8080, and 3) the following
// env vars set:
//
//	export INTEGTEST_DESTROY_DB_URL=postgres://postgres:postgres@localhost:5432/postgres
//	export INTEGTEST_DESTROY_DB_I_MEAN_IT=true
//	export INTEGTEST_SERVER=true
//
// NOTE that when Go runs tests, it runs every package in parallel (or, at least, it can).
// These integration tests follow the tip given here
// https://pkg.go.dev/testing@master#hdr-Subtests_and_Sub_benchmarks
// so that this entire "package" 1) is the ONLY source of integration tests (so other
// package tests can run in parallel because they have no state and cannot interfere
// with each other), and 2) runs all of its tests SERIALLY so that we can reason
// about the state of the database, which gets mutated during the test run.
// IF ANY OTHER INTEGRATION TESTS NEED TO MUTATE THE DATABASE, PLEASE ADD THOSE
// TESTS TO THIS FILE SO THAT INTEGRATION TESTS WILL CONTINUE TO RUN SERIALLY
// AND NOT TRIP OVER EACH OTHER!

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/manniwood/iidy-protobuf-dates/data"
	"github.com/manniwood/iidy-protobuf-dates/migrations"
	pb "github.com/manniwood/iidy-protobuf-dates/pb/iidy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// This is the only PUBLIC function that a run of
// `go test ./...` will find. All sub tests
// are run serially, in a controlled manner. However,
// use of t.Run() and other Go test facilities still
// makes this play nicely with Go's way of testing.
// See https://pkg.go.dev/testing@master#hdr-Subtests_and_Sub_benchmarks
// for more details.
func Test_Integrations(t *testing.T) {
	testDataFunctions(t)
	testServer(t)
}

func testDataFunctions(t *testing.T) {
	// INTEGTEST_DESTROY_DB_URL=postgres://postgres:postgres@localhost:5432/postgres
	dbURL := os.Getenv("INTEGTEST_DESTROY_DB_URL")
	if dbURL == "" {
		t.Skip("Not running data integration tests; INTEGTEST_DESTROY_DB_URL not set.")
	}

	ctx := context.Background()

	migrateConn, err := data.CreatePGXConnForMigration(ctx, dbURL)
	if err != nil {
		t.Fatalf("Could not create pgx conn for migration: %v", err)
	}
	defer migrateConn.Close(ctx)

	// Ensure the db is in a known state.
	wipeDB(ctx, t, migrateConn)
	// Clean up db when done.
	defer wipeDB(ctx, t, migrateConn)

	err = data.MigrateDB(ctx, migrateConn, migrations.Migrations, data.TernMigrationTable)
	if err != nil {
		t.Fatalf("Could not migrate db: %v", err)
	}

	pool, err := data.CreatePGXPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("Could not create pgx pool for integ tests: %v", err)
	}
	defer pool.Close()

	// Run these tests serially so that we always know
	// the state of the db.

	t.Run("InsertOne", func(t *testing.T) {
		count, err := data.InsertOne(context.Background(), pool, "downloads", "kernel.tar.gz")
		if err != nil {
			t.Errorf("Error adding item: %v", err)
		}
		if count != 1 {
			t.Error("Did not properly add item to list.")
		}
	})

	t.Run("GetOne", func(t *testing.T) {
		attempts, ok, err := data.GetOne(context.Background(), pool, "downloads", "kernel.tar.gz")
		if err != nil {
			t.Errorf("Error getting item: %v", err)
		}
		if attempts != 0 {
			t.Error("attempts != 0")
		}
		if !ok {
			t.Error("Did not properly add item to list.")
		}
	})

	t.Run("GetOne item does not exist", func(t *testing.T) {
		_, ok, err := data.GetOne(context.Background(), pool, "downloads", "I do not exist")
		if err != nil {
			t.Errorf("Error getting item: %v", err)
		}
		if ok {
			t.Error("List claims to return value that was not added to list.")
		}
	})

	t.Run("GetOne list does not exist", func(t *testing.T) {
		_, ok, err := data.GetOne(context.Background(), pool, "I do not exist", "kernel.tar.gz")
		if err != nil {
			t.Errorf("Error getting item: %v", err)
		}
		if ok {
			t.Error("Non-existent list claims to return value.")
		}
	})

	t.Run("DeleteOne", func(t *testing.T) {
		count, err := data.DeleteOne(context.Background(), pool, "downloads", "kernel.tar.gz")
		if err != nil {
			t.Errorf("Error trying to delete item from list: %v", err)
		}
		if count != 1 {
			t.Error("Did not properly delete item from list.")
		}
	})

	t.Run("GetOne should fail on deleted item", func(t *testing.T) {
		_, ok, err := data.GetOne(context.Background(), pool, "downloads", "kernel.tar.gz")
		if err != nil {
			t.Errorf("Error getting item: %v", err)
		}
		if ok {
			t.Error("Did not properly delete item to list.")
		}
	})

	t.Run("DeleteOne item was not there in the first place", func(t *testing.T) {
		count, err := data.DeleteOne(context.Background(), pool, "downloads", "I do not exist")
		if err != nil {
			t.Errorf("Error trying to delete item from list: %v", err)
		}
		if count != 0 {
			t.Error("Did not properly report non-deletion of item.")
		}
	})

	t.Run("DeleteOne list was not there in the first place", func(t *testing.T) {
		count, err := data.DeleteOne(context.Background(), pool, "I do not exist", "kernel.tar.gz")
		if err != nil {
			t.Errorf("Error trying to delete item from non-existent list: %v", err)
		}
		if count != 0 {
			t.Error("Did not properly report non-deletion of item from no-existent list.")
		}
	})

	t.Run("InsertOne for incrementing", func(t *testing.T) {
		count, err := data.InsertOne(context.Background(), pool, "downloads", "kernel.tar.gz")
		if err != nil {
			t.Errorf("Error adding item: %v", err)
		}
		if count != 1 {
			t.Error("Did not properly add item to list.")
		}
	})

	t.Run("IncrementOne", func(t *testing.T) {
		count, err := data.IncrementOne(context.Background(), pool, "downloads", "kernel.tar.gz")
		if err != nil {
			t.Errorf("Error trying to increment: %v", err)
		}
		if count != 1 {
			t.Error("Did not properly increment.")
		}
	})

	t.Run("GetOne that has been incremented", func(t *testing.T) {
		attempts, ok, err := data.GetOne(context.Background(), pool, "downloads", "kernel.tar.gz")
		if err != nil {
			t.Errorf("Error getting item: %v", err)
		}
		if !ok {
			t.Error("Did not properly add item to list.")
		}
		if attempts != 1 {
			t.Error("Did not properly increment item in list.")
		}
	})

	t.Run("IncrementOne item does not exist", func(t *testing.T) {
		count, err := data.IncrementOne(context.Background(), pool, "downloads", "I do not exist")
		if err != nil {
			t.Errorf("Error trying to increment item from list: %v", err)
		}
		if count != 0 {
			t.Error("Did not properly report non-increment of item.")
		}
	})

	t.Run("IncrementOne list does not exist", func(t *testing.T) {
		count, err := data.IncrementOne(context.Background(), pool, "I do not exist", "kernel.tar.gz")
		if err != nil {
			t.Errorf("Error trying to increment item from list: %v", err)
		}
		if count != 0 {
			t.Error("Did not properly report non-increment of item from non-existent list.")
		}
	})

	t.Run("DeleteOne Starting Fresh", func(t *testing.T) {
		count, err := data.DeleteOne(context.Background(), pool, "downloads", "kernel.tar.gz")
		if err != nil {
			t.Errorf("Error trying to delete item from list: %v", err)
		}
		if count != 1 {
			t.Error("Did not properly delete item from list.")
		}
	})

	testFiles := []string{"kernel.tar.gz", "vim.tar.gz", "robots.txt"}

	t.Run("InsertBatch", func(t *testing.T) {
		count, err := data.InsertBatch(context.Background(), pool, "downloads", testFiles)
		if err != nil {
			t.Errorf("Error batch inserting: %v", err)
		}
		if count != 3 {
			t.Errorf("Batch incremented wrong number of items. Expected 5, got %v", count)
		}

		// If we get the list items, do they exist?
		for _, file := range testFiles {
			attempts, ok, err := data.GetOne(context.Background(), pool, "downloads", file)
			if err != nil {
				t.Errorf("Error getting item: %v", err)
			}
			if attempts != 0 {
				t.Errorf("Attempts for freshly-created %v is not 0", file)
			}
			if !ok {
				t.Error("Did not properly add item to list.")
			}
		}
	})

	t.Run("InsertBatch nothing", func(t *testing.T) {
		count, err := data.InsertBatch(context.Background(), pool, "downloads", []string{})
		if err != nil {
			t.Errorf("Error batch inserting: %v", err)
		}
		if count != 0 {
			t.Errorf("Batch added wrong number of items. Expected 0, got %v", count)
		}
	})

	t.Run("DeleteBatch", func(t *testing.T) {
		count, err := data.DeleteBatch(context.Background(), pool, "downloads", testFiles)
		if err != nil {
			t.Errorf("Error batch deleting: %v", err)
		}
		if count != int64(len(testFiles)) {
			t.Errorf("Batch deleted wrong number of items. Expected %d, got %v", len(testFiles), count)
		}
	})

	t.Run("DeleteBatch partial", func(t *testing.T) {
		// Batch add a bunch of test items.
		files := []string{"a", "b", "c", "d", "e", "f", "g"}
		count, err := data.InsertBatch(context.Background(), pool, "downloads", files)
		if err != nil {
			t.Errorf("Error batch inserting: %v", err)
		}
		if count != 7 {
			t.Errorf("Batch added wrong number of items. Expected 7, got %v", count)
		}

		// Does batch delete work?
		count, err = data.DeleteBatch(context.Background(), pool, "downloads", []string{"a", "b", "c", "d", "e"})
		if err != nil {
			t.Errorf("Error batch deleting: %v", err)
		}
		if count != 5 {
			t.Errorf("Batch deleted wrong number of items. Expected 5, got %v", count)
		}

		// If we look for the deleted items, are they correctly missing?
		for _, file := range []string{"a", "b", "c", "d", "e"} {
			_, ok, err := data.GetOne(context.Background(), pool, "downloads", file)
			if err != nil {
				t.Errorf("Error getting item: %v", err)
			}
			if ok {
				t.Errorf("Found item %v that should have been deleted from list.", file)
			}
		}

		// Were other items left alone?
		for _, file := range []string{"f", "g"} {
			attempts, ok, err := data.GetOne(context.Background(), pool, "downloads", file)
			if err != nil {
				t.Errorf("Error getting item: %v", err)
			}
			if !ok {
				t.Errorf("Item %v should not have been deleted from list.", file)
			}
			if attempts != 0 {
				t.Errorf("Item %v is incorrectly incremented.", file)
			}
		}

		// Now just delete remaining, to clear for next test
		count, err = data.DeleteBatch(context.Background(), pool, "downloads", []string{"f", "g"})
		if err != nil {
			t.Errorf("Error batch deleting: %v", err)
		}
		if count != 2 {
			t.Errorf("Batch deleted wrong number of items. Expected 2, got %v", count)
		}
	})

	t.Run("GetBatch", func(t *testing.T) {
		// Batch add a bunch of test items.
		files := []string{"a", "b", "c", "d", "e", "f", "g"}
		count, err := data.InsertBatch(context.Background(), pool, "downloads", files)
		if err != nil {
			t.Errorf("Error batch inserting: %v", err)
		}
		if count != 7 {
			t.Errorf("Batch added wrong number of items. Expected 5, got %v", count)
		}

		var tests = []struct {
			afterItem string
			want      []*pb.ItemAttempt
		}{
			{"", []*pb.ItemAttempt{{Item: "a", Attempts: 0}, {Item: "b", Attempts: 0}}},
			{"b", []*pb.ItemAttempt{{Item: "c", Attempts: 0}, {Item: "d", Attempts: 0}}},
			{"d", []*pb.ItemAttempt{{Item: "e", Attempts: 0}, {Item: "f", Attempts: 0}}},
			{"f", []*pb.ItemAttempt{{Item: "g", Attempts: 0}}},
		}

		// If we batch get 2 items at a time, does everything work?
		for _, test := range tests {
			items, err := data.GetBatch(context.Background(), pool, "downloads", test.afterItem, 2)
			if err != nil {
				t.Errorf("Error batch fetching: %v", err)
			}
			if !reflect.DeepEqual(test.want, items) {
				t.Errorf("Expected %v; got %v", test.want, items)
			}
		}

		// What if we batch get nothing?
		items, err := data.GetBatch(context.Background(), pool, "downloads", "", 0)
		if err != nil {
			t.Errorf("Error batch deleting: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("Batch get of nothing yeilded results!")
		}

		// Now just delete remaining, to clear for next test
		count, err = data.DeleteBatch(context.Background(), pool, "downloads", files)
		if err != nil {
			t.Errorf("Error batch deleting: %v", err)
		}
		if count != int64(len(files)) {
			t.Errorf("Batch deleted wrong number of items. Expected %d, got %v", len(files), count)
		}
	})

	t.Run("IncrementBatch", func(t *testing.T) {
		// Batch add a bunch of test items.
		files := []string{"a", "b", "c", "d", "e", "f", "g"}
		count, err := data.InsertBatch(context.Background(), pool, "downloads", files)
		if err != nil {
			t.Errorf("Error batch inserting: %v", err)
		}
		if count != 7 {
			t.Errorf("Batch added wrong number of items. Expected 5, got %v", count)
		}

		// Does batch increment work?
		count, err = data.IncrementBatch(context.Background(), pool, "downloads", []string{"a", "b", "c", "d", "e"})
		if err != nil {
			t.Errorf("Error batch incrementing: %v", err)
		}
		if count != 5 {
			t.Errorf("Batch incremented wrong number of items. Expected 5, got %v", count)
		}

		// If we look for incremented items, are they incremented?
		for _, file := range []string{"a", "b", "c", "d", "e"} {
			attempts, ok, err := data.GetOne(context.Background(), pool, "downloads", file)
			if err != nil {
				t.Errorf("Error getting item: %v", err)
			}
			if !ok {
				t.Errorf("Did not properly get item %v from list.", file)
			}
			if attempts != 1 {
				t.Errorf("Did not properly increment item %v.", file)
			}
		}

		// What about non-incremented items? Were they left alone?
		for _, file := range []string{"f", "g"} {
			attempts, ok, err := data.GetOne(context.Background(), pool, "downloads", file)
			if err != nil {
				t.Errorf("Error getting item: %v", err)
			}
			if !ok {
				t.Errorf("Did not properly get item %v from list.", file)
			}
			if attempts != 0 {
				t.Errorf("Item %v is incorrectly incremented.", file)
			}
		}

		// What if we batch increment nothing?
		count, err = data.IncrementBatch(context.Background(), pool, "downloads", []string{})
		if err != nil {
			t.Errorf("Error batch deleting: %v", err)
		}
		if count != 0 {
			t.Errorf("Batch incremented wrong number of items. Expected 0, got %v", count)
		}

		// Now just delete remaining, to clear for next test
		count, err = data.DeleteBatch(context.Background(), pool, "downloads", files)
		if err != nil {
			t.Errorf("Error batch deleting: %v", err)
		}
		if count != int64(len(files)) {
			t.Errorf("Batch deleted wrong number of items. Expected %d, got %v", len(files), count)
		}
	})
}

func testServer(t *testing.T) {

	runServerTests := os.Getenv("INTEGTEST_SERVER")
	if runServerTests != "true" {
		t.Skip("Not running server integration tests; INTEGTEST_SERVER != true.")
	}

	const defaultPort string = "8080"
	port := os.Getenv("IIDY_PORT")
	if port == "" {
		port = defaultPort
	}

	// Create a gRPC client channel. RPCs will connect lazily when invoked.
	conn, err := grpc.NewClient(
		"localhost:"+port,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to create grpc client: %v", err)
	}
	defer conn.Close()

	// Create a client stub
	client := pb.NewIIDYServiceClient(conn)
	// This is a little strange. Even though the server migrates the db
	// at startup, we want to be sure the db is in a known state, so we
	// wipe it clean and do a fresh migration. This essentially "pulls the
	// rug out from under the service", but because we are the only client
	// of the service, we can do this.

	// INTEGTEST_DESTROY_DB_URL=postgres://postgres:postgres@localhost:5432/postgres
	dbURL := os.Getenv("INTEGTEST_DESTROY_DB_URL")
	if dbURL == "" {
		t.Skip("Not running data integration tests; INTEGTEST_DESTROY_DB_URL not set.")
	}

	ctx := context.Background()

	migrateConn, err := data.CreatePGXConnForMigration(ctx, dbURL)
	if err != nil {
		t.Fatalf("Could not create pgx conn for migration: %v", err)
	}
	defer migrateConn.Close(ctx)

	// Ensure the db is in a known state.
	wipeDB(ctx, t, migrateConn)
	// Clean up db when done.
	defer wipeDB(ctx, t, migrateConn)

	err = data.MigrateDB(ctx, migrateConn, migrations.Migrations, data.TernMigrationTable)
	if err != nil {
		t.Fatalf("Could not migrate db: %v", err)
	}

	// Run these tests serially so that we always know
	// the state of the db behind the service.

	t.Run("InsertOne", func(t *testing.T) {
		req := &pb.AddListItemRequest{
			ListItem: &pb.ListItem{
				List: "downloads",
				Item: "kernel.tar.gz",
			},
		}
		resp, err := client.AddListItem(ctx, req)
		if err != nil {
			t.Errorf("Error adding item: %v", err)
		}
		got := resp.GetAdded()
		var want int64 = 1
		if got != want {
			t.Errorf("Add was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("GetOne", func(t *testing.T) {
		req := &pb.GetListItemRequest{
			ListItem: &pb.ListItem{
				List: "downloads",
				Item: "kernel.tar.gz",
			},
		}
		resp, err := client.GetListItem(ctx, req)
		if err != nil {
			t.Errorf("Error getting item: %v", err)
		}
		got := resp.GetAttempts()
		var want int32 = 0
		if got != want {
			t.Errorf("Get was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("GetOne item that does not exist", func(t *testing.T) {
		req := &pb.GetListItemRequest{
			ListItem: &pb.ListItem{
				List: "downloads",
				Item: "I_do_not_exist",
			},
		}
		_, err := client.GetListItem(ctx, req)
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				want := codes.NotFound
				got := st.Code()
				if got != want {
					t.Errorf("Expected code %v but got code %v", want, got)
				}
			} else {
				t.Errorf("A non-gRPC error occurred: %v", err)
			}
		} else {
			t.Errorf("Error was nil but needed to exist")
		}
	})

	t.Run("GetOne where list does not exist", func(t *testing.T) {
		req := &pb.GetListItemRequest{
			ListItem: &pb.ListItem{
				List: "I_do_not_exist",
				Item: "kernel.tar.gz",
			},
		}
		_, err := client.GetListItem(ctx, req)
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				want := codes.NotFound
				got := st.Code()
				if got != want {
					t.Errorf("Expected code %v but got code %v", want, got)
				}
			} else {
				t.Errorf("A non-gRPC error occurred: %v", err)
			}
		} else {
			t.Errorf("Error was nil but needed to exist")
		}
	})

	t.Run("DeleteOne", func(t *testing.T) {
		req := &pb.DeleteListItemRequest{
			ListItem: &pb.ListItem{
				List: "downloads",
				Item: "kernel.tar.gz",
			},
		}
		resp, err := client.DeleteListItem(ctx, req)
		if err != nil {
			t.Errorf("Error adding item: %v", err)
		}
		got := resp.GetDeleted()
		var want int64 = 1
		if got != want {
			t.Errorf("Delete was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("GetOne should fail on deleted item", func(t *testing.T) {
		req := &pb.GetListItemRequest{
			ListItem: &pb.ListItem{
				List: "downloads",
				Item: "kernel.tar.gz",
			},
		}
		_, err := client.GetListItem(ctx, req)
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				want := codes.NotFound
				got := st.Code()
				if got != want {
					t.Errorf("Expected code %v but got code %v", want, got)
				}
			} else {
				t.Errorf("A non-gRPC error occurred: %v", err)
			}
		} else {
			t.Errorf("Error was nil but needed to exist")
		}
	})

	t.Run("DeleteOne item was not there in the first place", func(t *testing.T) {
		req := &pb.DeleteListItemRequest{
			ListItem: &pb.ListItem{
				List: "downloads",
				Item: "I_do_not_exist",
			},
		}
		resp, err := client.DeleteListItem(ctx, req)
		if err != nil {
			t.Errorf("Error adding item: %v", err)
		}
		got := resp.GetDeleted()
		var want int64 = 0
		if got != want {
			t.Errorf("Delete was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("DeleteOne list was not there in the first place", func(t *testing.T) {
		req := &pb.DeleteListItemRequest{
			ListItem: &pb.ListItem{
				List: "I_do_not_exist",
				Item: "kernel.tar.gz",
			},
		}
		resp, err := client.DeleteListItem(ctx, req)
		if err != nil {
			t.Errorf("Error adding item: %v", err)
		}
		got := resp.GetDeleted()
		var want int64 = 0
		if got != want {
			t.Errorf("Delete was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("InsertOne for incrementing", func(t *testing.T) {
		req := &pb.AddListItemRequest{
			ListItem: &pb.ListItem{
				List: "downloads",
				Item: "kernel.tar.gz",
			},
		}
		resp, err := client.AddListItem(ctx, req)
		if err != nil {
			t.Errorf("Error adding item: %v", err)
		}
		got := resp.GetAdded()
		var want int64 = 1
		if got != want {
			t.Errorf("Add was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("IncrementOne", func(t *testing.T) {
		req := &pb.IncrementListItemRequest{
			ListItem: &pb.ListItem{
				List: "downloads",
				Item: "kernel.tar.gz",
			},
		}
		resp, err := client.IncrementListItem(ctx, req)
		if err != nil {
			t.Errorf("Error adding item: %v", err)
		}
		got := resp.GetIncremented()
		var want int64 = 1
		if got != want {
			t.Errorf("Increment was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("GetOne that has been incremented", func(t *testing.T) {
		req := &pb.GetListItemRequest{
			ListItem: &pb.ListItem{
				List: "downloads",
				Item: "kernel.tar.gz",
			},
		}
		resp, err := client.GetListItem(ctx, req)
		if err != nil {
			t.Errorf("Error getting item: %v", err)
		}
		got := resp.GetAttempts()
		var want int32 = 1
		if got != want {
			t.Errorf("Get was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("IncrementOne item that does not exist", func(t *testing.T) {
		req := &pb.IncrementListItemRequest{
			ListItem: &pb.ListItem{
				List: "downloads",
				Item: "I_do_not_exist",
			},
		}
		resp, err := client.IncrementListItem(ctx, req)
		if err != nil {
			t.Errorf("Error incrementing item: %v", err)
		}
		got := resp.GetIncremented()
		var want int64 = 0
		if got != want {
			t.Errorf("Get was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("IncrementOne where list does not exist", func(t *testing.T) {
		req := &pb.IncrementListItemRequest{
			ListItem: &pb.ListItem{
				List: "I_do_not_exist",
				Item: "kernel.tar.gz",
			},
		}
		resp, err := client.IncrementListItem(ctx, req)
		if err != nil {
			t.Errorf("Error incrementing item: %v", err)
		}
		got := resp.GetIncremented()
		var want int64 = 0
		if got != want {
			t.Errorf("Get was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("DeleteOne Starting Fresh", func(t *testing.T) {
		req := &pb.DeleteListItemRequest{
			ListItem: &pb.ListItem{
				List: "downloads",
				Item: "kernel.tar.gz",
			},
		}
		resp, err := client.DeleteListItem(ctx, req)
		if err != nil {
			t.Errorf("Error adding item: %v", err)
		}
		got := resp.GetDeleted()
		var want int64 = 1
		if got != want {
			t.Errorf("Delete was supposed to have been %d but was %d instead", want, got)
		}
	})

	testFiles := []string{"kernel.tar.gz", "vim.tar.gz", "robots.txt"}

	t.Run("InsertBatch", func(t *testing.T) {
		req := &pb.AddListItemsRequest{
			List:  "downloads",
			Items: testFiles,
		}
		resp, err := client.AddListItems(ctx, req)
		if err != nil {
			t.Errorf("Error adding items: %v", err)
		}
		got := resp.GetAdded()
		var want int64 = 3
		if got != want {
			t.Errorf("Add was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("InsertBatch of zero files", func(t *testing.T) {
		req := &pb.AddListItemsRequest{
			List:  "downloads",
			Items: []string{},
		}
		_, err := client.AddListItems(ctx, req)
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				want := codes.InvalidArgument
				got := st.Code()
				if got != want {
					t.Errorf("Expected code %v but got code %v", want, got)
				}
			} else {
				t.Errorf("A non-gRPC error occurred: %v", err)
			}
		} else {
			t.Errorf("Error was nil but needed to exist")
		}
	})

	t.Run("DeleteBatch", func(t *testing.T) {
		req := &pb.DeleteListItemsRequest{
			List:  "downloads",
			Items: testFiles,
		}
		resp, err := client.DeleteListItems(ctx, req)
		if err != nil {
			t.Errorf("Error adding items: %v", err)
		}
		got := resp.GetDeleted()
		var want int64 = 3
		if got != want {
			t.Errorf("Delete was supposed to have been %d but was %d instead", want, got)
		}
	})

	t.Run("DeleteBatch partial", func(t *testing.T) {
		// Batch add a bunch of test items.
		files := []string{"a", "b", "c", "d", "e", "f", "g"}

		req := &pb.AddListItemsRequest{
			List:  "downloads",
			Items: files,
		}
		resp, err := client.AddListItems(ctx, req)
		if err != nil {
			t.Errorf("Error adding items: %v", err)
		}
		got := resp.GetAdded()
		var want int64 = 7
		if got != want {
			t.Errorf("Add was supposed to have been %d but was %d instead", want, got)
		}

		// Does batch delete work?
		files2 := []string{"a", "b", "c", "d", "e"}
		req2 := &pb.DeleteListItemsRequest{
			List:  "downloads",
			Items: files2,
		}
		resp2, err := client.DeleteListItems(ctx, req2)
		if err != nil {
			t.Errorf("Error adding items: %v", err)
		}
		got2 := resp2.GetDeleted()
		var want2 int64 = 5
		if got2 != want2 {
			t.Errorf("Delete was supposed to have been %d but was %d instead", want2, got2)
		}

		// If we look for the deleted items, are they correctly missing?
		for _, file := range []string{"a", "b", "c", "d", "e"} {
			req := &pb.GetListItemRequest{
				ListItem: &pb.ListItem{
					List: "downloads",
					Item: file,
				},
			}
			_, err := client.GetListItem(ctx, req)
			if err != nil {
				st, ok := status.FromError(err)
				if ok {
					want := codes.NotFound
					got := st.Code()
					if got != want {
						t.Errorf("Expected code %v but got code %v", want, got)
					}
				} else {
					t.Errorf("A non-gRPC error occurred: %v", err)
				}
			} else {
				t.Errorf("Error was nil but needed to exist")
			}
		}

		// Were other items left alone?
		for _, file := range []string{"f", "g"} {
			req := &pb.GetListItemRequest{
				ListItem: &pb.ListItem{
					List: "downloads",
					Item: file,
				},
			}
			resp, err := client.GetListItem(ctx, req)
			if err != nil {
				t.Errorf("Error getting item: %v", err)
			}
			got := resp.GetAttempts()
			var want int32 = 0
			if got != want {
				t.Errorf("Get was supposed to have been %d but was %d instead", want, got)
			}
		}

		// Now just delete remaining, to clear for next test
		files3 := []string{"f", "g"}
		req3 := &pb.DeleteListItemsRequest{
			List:  "downloads",
			Items: files3,
		}
		resp3, err := client.DeleteListItems(ctx, req3)
		if err != nil {
			t.Errorf("Error adding items: %v", err)
		}
		got3 := resp3.GetDeleted()
		var want3 int64 = 2
		if got3 != want3 {
			t.Errorf("Delete was supposed to have been %d but was %d instead", want3, got3)
		}
	})

	t.Run("GetBatch", func(t *testing.T) {
		// Batch add a bunch of test items.
		files := []string{"a", "b", "c", "d", "e", "f", "g"}

		req := &pb.AddListItemsRequest{
			List:  "downloads",
			Items: files,
		}
		resp, err := client.AddListItems(ctx, req)
		if err != nil {
			t.Errorf("Error adding items: %v", err)
		}
		got := resp.GetAdded()
		var want int64 = 7
		if got != want {
			t.Errorf("Add was supposed to have been %d but was %d instead", want, got)
		}

		var tests = []struct {
			afterItem string
			want      []*pb.ItemAttempt
		}{
			{"", []*pb.ItemAttempt{&pb.ItemAttempt{Item: "a", Attempts: 0}, &pb.ItemAttempt{Item: "b", Attempts: 0}}},
			{"b", []*pb.ItemAttempt{&pb.ItemAttempt{Item: "c", Attempts: 0}, &pb.ItemAttempt{Item: "d", Attempts: 0}}},
			{"d", []*pb.ItemAttempt{&pb.ItemAttempt{Item: "e", Attempts: 0}, &pb.ItemAttempt{Item: "f", Attempts: 0}}},
			{"f", []*pb.ItemAttempt{&pb.ItemAttempt{Item: "g", Attempts: 0}}},
		}

		// If we batch get 2 items at a time, does everything work?
		for _, test := range tests {
			req := &pb.GetListItemsRequest{
				List:      "downloads",
				Amount:    2,
				AfterItem: test.afterItem,
			}
			resp, err := client.GetListItems(ctx, req)
			if err != nil {
				t.Errorf("Error getting items: %v", err)
			}

			if !reflect.DeepEqual(resp.GetItems(), test.want) {
				t.Errorf("fetched items where supposed to be %v but were %v instead", test.want, resp.GetItems())
			}
		}

		// What if we batch get nothing?
		req2 := &pb.GetListItemsRequest{
			List:      "downloads",
			Amount:    0,
			AfterItem: "",
		}
		resp2, err := client.GetListItems(ctx, req2)
		if err != nil {
			t.Errorf("Error getting items: %v", err)
		}

		if len(resp2.GetItems()) != 0 {
			t.Error("Supposed to get 0 fetched items, but got more than 0")
		}

		// Now just delete remaining, to clear for next test
		files3 := []string{"a", "b", "c", "d", "e", "f", "g"}
		req3 := &pb.DeleteListItemsRequest{
			List:  "downloads",
			Items: files3,
		}
		resp3, err := client.DeleteListItems(ctx, req3)
		if err != nil {
			t.Errorf("Error deleting items: %v", err)
		}
		if resp3.GetDeleted() != int64(len(files3)) {
			t.Errorf("Delete was supposed to have been %d but was %d instead", len(files3), resp3.GetDeleted())
		}
	})

	t.Run("IncrementBatch", func(t *testing.T) {
		// Batch add a bunch of test items.
		files := []string{"a", "b", "c", "d", "e", "f", "g"}

		req := &pb.AddListItemsRequest{
			List:  "downloads",
			Items: files,
		}
		resp, err := client.AddListItems(ctx, req)
		if err != nil {
			t.Errorf("Error adding items: %v", err)
		}
		got := resp.GetAdded()
		var want int64 = 7
		if got != want {
			t.Errorf("Add was supposed to have been %d but was %d instead", want, got)
		}

		// Does batch increment work?
		files2 := []string{"a", "b", "c", "d", "e"}

		req2 := &pb.IncrementListItemsRequest{
			List:  "downloads",
			Items: files2,
		}
		resp2, err := client.IncrementListItems(ctx, req2)
		if err != nil {
			t.Errorf("Error incrementing items: %v", err)
		}
		got2 := resp2.GetIncremented()
		var want2 int64 = 5
		if got2 != want2 {
			t.Errorf("Increment was supposed to have been %d but was %d instead", want2, got2)
		}

		// If we look for incremented items, are they incremented?
		for _, file := range []string{"a", "b", "c", "d", "e"} {
			req := &pb.GetListItemRequest{
				ListItem: &pb.ListItem{
					List: "downloads",
					Item: file,
				},
			}
			resp, err := client.GetListItem(ctx, req)
			if err != nil {
				t.Errorf("Error getting item: %v", err)
			}
			got := resp.GetAttempts()
			var want int32 = 1
			if got != want {
				t.Errorf("Get was supposed to have been %d but was %d instead", want, got)
			}
		}

		// What about non-incremented items? Were they left alone?
		for _, file := range []string{"f", "g"} {
			req := &pb.GetListItemRequest{
				ListItem: &pb.ListItem{
					List: "downloads",
					Item: file,
				},
			}
			resp, err := client.GetListItem(ctx, req)
			if err != nil {
				t.Errorf("Error getting item: %v", err)
			}
			got := resp.GetAttempts()
			var want int32 = 0
			if got != want {
				t.Errorf("Get was supposed to have been %d but was %d instead", want, got)
			}
		}

		// What if we batch increment nothing?
		req3 := &pb.IncrementListItemsRequest{
			List:  "downloads",
			Items: []string{},
		}
		_, err = client.IncrementListItems(ctx, req3)
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				want := codes.InvalidArgument
				got := st.Code()
				if got != want {
					t.Errorf("Expected code %v but got code %v", want, got)
				}
			} else {
				t.Errorf("A non-gRPC error occurred: %v", err)
			}
		} else {
			t.Errorf("Error was nil but needed to exist")
		}

		// Now just delete remaining, to clear for next test
		files4 := []string{"a", "b", "c", "d", "e", "f", "g"}
		req4 := &pb.DeleteListItemsRequest{
			List:  "downloads",
			Items: files4,
		}
		resp4, err := client.DeleteListItems(ctx, req4)
		if err != nil {
			t.Errorf("Error deleting items: %v", err)
		}
		if resp4.GetDeleted() != int64(len(files4)) {
			t.Errorf("Delete was supposed to have been %d but was %d instead", len(files4), resp4.GetDeleted())
		}
	})
}

func wipeDB(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	nukeDBOK := os.Getenv("INTEGTEST_DESTROY_DB_I_MEAN_IT")
	if nukeDBOK != "true" {
		t.Skip("Not running integration tests and not destroying db; INTEGTEST_DESTROY_DB_I_MEAN_IT != true.")
	}

	// Drop entire iidy schema.
	_, err := conn.Exec(ctx, "drop schema if exists iidy cascade")
	if err != nil {
		t.Fatalf("Could not destroy iidy schema : %v", err)
	}

	// Drop special tern table.
	_, err = conn.Exec(ctx, fmt.Sprintf("drop table if exists %s", data.TernMigrationTable))
	if err != nil {
		t.Fatalf("Could not drop tern migration table \"%s\": %v", data.TernMigrationTable, err)
	}
}
