package service

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manniwood/iidy-protobuf-dates/data"
	pb "github.com/manniwood/iidy-protobuf-dates/pb/iidy"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type IIDYService struct {
	pb.UnimplementedIIDYServiceServer

	Pool *pgxpool.Pool
}

func NewIIDYService(pool *pgxpool.Pool) *IIDYService {
	return &IIDYService{
		Pool: pool,
	}
}

func (s *IIDYService) AddListItem(ctx context.Context, req *pb.AddListItemRequest) (*pb.AddListItemResponse, error) {
	if req.GetListItem() == nil {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List")
	}
	list := req.GetListItem().GetList()
	if list == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List name")
	}
	item := req.GetListItem().GetItem()
	if item == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List item")
	}
	count, err := data.InsertOne(ctx, data.PgxPool, list, item)
	if err != nil {
		log.Printf("AddListItem %v: %v", req, err)
		return nil, status.Error(codes.Internal, "Problem trying to insert list item")
	}
	return &pb.AddListItemResponse{
		Added: count,
	}, nil
}

func (s *IIDYService) GetListItem(ctx context.Context, req *pb.GetListItemRequest) (*pb.GetListItemResponse, error) {
	if req.GetListItem() == nil {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List")
	}
	list := req.GetListItem().GetList()
	if list == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List name")
	}
	item := req.GetListItem().GetItem()
	if item == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List item")
	}
	attempts, ok, err := data.GetOne(ctx, data.PgxPool, list, item)
	if err != nil {
		log.Printf("GetListItem %v: %v", req, err)
		return nil, status.Error(codes.Internal, "Problem trying to get list item")
	}
	if !ok {
		return nil, status.Errorf(codes.NotFound, "Item %v from list %v not found", item, list)
	}
	return &pb.GetListItemResponse{
		Attempts: attempts,
	}, nil
}

func (s *IIDYService) IncrementListItem(ctx context.Context, req *pb.IncrementListItemRequest) (*pb.IncrementListItemResponse, error) {
	if req.GetListItem() == nil {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List")
	}
	list := req.GetListItem().GetList()
	if list == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List name")
	}
	item := req.GetListItem().GetItem()
	if item == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List item")
	}
	count, err := data.IncrementOne(ctx, data.PgxPool, list, item)
	if err != nil {
		log.Printf("IncrementListItem %v: %v", req, err)
		return nil, status.Error(codes.Internal, "Problem trying to increment list item")
	}
	return &pb.IncrementListItemResponse{
		Incremented: count,
	}, nil
}

func (s *IIDYService) DeleteListItem(ctx context.Context, req *pb.DeleteListItemRequest) (*pb.DeleteListItemResponse, error) {
	if req.GetListItem() == nil {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List")
	}
	list := req.GetListItem().GetList()
	if list == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List name")
	}
	item := req.GetListItem().GetItem()
	if item == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List item")
	}
	count, err := data.DeleteOne(ctx, data.PgxPool, list, item)
	if err != nil {
		log.Printf("DeleteListItem %v: %v", req, err)
		return nil, status.Error(codes.Internal, "Problem trying to delete list item")
	}
	return &pb.DeleteListItemResponse{
		Deleted: count,
	}, nil
}

func (s *IIDYService) AddListItems(ctx context.Context, req *pb.AddListItemsRequest) (*pb.AddListItemsResponse, error) {
	list := req.GetList()
	if list == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List name")
	}
	items := req.GetItems()
	// Note: nil items returns len 0 instead of NPE
	if len(items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List of items")
	}
	count, err := data.InsertBatch(ctx, data.PgxPool, list, items)
	if err != nil {
		log.Printf("AddListItems %v: %v", req, err)
		return nil, status.Error(codes.Internal, "Problem trying to insert list items")
	}
	return &pb.AddListItemsResponse{
		Added: count,
	}, nil
}

func (s *IIDYService) GetListItems(ctx context.Context, req *pb.GetListItemsRequest) (*pb.GetListItemsResponse, error) {
	list := req.GetList()
	if list == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List name")
	}
	amount := req.GetAmount()
	if list == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide an amount (batch size)")
	}
	// Note: req.GetAfterItem() can be left empty if the user wants items from the start of the list
	afterItem := req.GetAfterItem()
	itemAttempts, err := data.GetBatch(ctx, data.PgxPool, list, afterItem, amount)
	if err != nil {
		log.Printf("GetListItems %v: %v", req, err)
		return nil, status.Error(codes.Internal, "Problem trying to get list items")
	}
	return &pb.GetListItemsResponse{
		Items: itemAttempts,
	}, nil
}

func (s *IIDYService) IncrementListItems(ctx context.Context, req *pb.IncrementListItemsRequest) (*pb.IncrementListItemsResponse, error) {
	list := req.GetList()
	if list == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List name")
	}
	items := req.GetItems()
	// Note: nil items returns len 0 instead of NPE
	if len(items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List of items")
	}
	count, err := data.IncrementBatch(ctx, data.PgxPool, list, items)
	if err != nil {
		log.Printf("IncrementListItems %v: %v", req, err)
		return nil, status.Error(codes.Internal, "Problem trying to increment list items")
	}
	return &pb.IncrementListItemsResponse{
		Incremented: count,
	}, nil
}

func (s *IIDYService) DeleteListItems(ctx context.Context, req *pb.DeleteListItemsRequest) (*pb.DeleteListItemsResponse, error) {
	list := req.GetList()
	if list == "" {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List name")
	}
	items := req.GetItems()
	// Note: nil items returns len 0 instead of NPE
	if len(items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Must provide a List of items")
	}
	count, err := data.DeleteBatch(ctx, data.PgxPool, list, items)
	if err != nil {
		log.Printf("DeleteListItems %v: %v", req, err)
		return nil, status.Error(codes.Internal, "Problem trying to increment list items")
	}
	return &pb.DeleteListItemsResponse{
		Deleted: count,
	}, nil
}
