package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	desc "github.com/DmitryKovganov/go-microservices-auth/pkg/user_v1"
)

const grpcPort = 50051

type server struct {
	desc.UnimplementedUserV1Server
}

type SyncMap struct {
	users map[int64]*desc.User
	m     sync.RWMutex
}

var state = &SyncMap{
	users: make(map[int64]*desc.User),
}

func (s *server) Create(ctx context.Context, req *desc.CreateRequest) (*desc.CreateResponse, error) {
	log.Printf("Create, req: %+v", req)

	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Name is required")
	}

	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "Email is required")
	}

	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "Password is required")
	}

	if req.GetRole() == desc.UserRole_UNKNOWN {
		return nil, status.Error(codes.InvalidArgument, "Role is required")
	}

	if req.GetPasswordConfirm() == "" {
		return nil, status.Error(codes.InvalidArgument, "PasswordConfirm is required")
	}

	if req.GetPassword() != req.GetPasswordConfirm() {
		return nil, status.Error(codes.InvalidArgument, "PasswordConfirm and Password are not equal")
	}

	now := timestamppb.Now()
	createdUser := &desc.User{
		Id: rand.Int63(),
		Info: &desc.UserInfo{
			Name:     req.Name,
			Email:    req.Email,
			Password: req.Password,
			Role:     req.Role,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	state.m.Lock()
	defer state.m.Unlock()

	state.users[createdUser.Id] = createdUser

	return &desc.CreateResponse{
		Id: createdUser.Id,
	}, nil
}

func (s *server) Get(ctx context.Context, req *desc.GetRequest) (*desc.GetResponse, error) {
	log.Printf("Get, req: %+v", req)

	if req.GetId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "Id is required")
	}

	state.m.RLock()
	defer state.m.RLock()

	user, ok := state.users[req.Id]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "User not found")
	}

	return &desc.GetResponse{
		Id:        user.Id,
		Name:      user.Info.Name,
		Email:     user.Info.Email,
		Role:      user.Info.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

func (s *server) Update(ctx context.Context, req *desc.UpdateRequest) (*emptypb.Empty, error) {
	log.Printf("Update, req: %+v", req)

	if req.GetId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "Id is required")
	}

	userId := req.GetId()

	state.m.RLock()
	defer state.m.RLock()

	user, ok := state.users[userId]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "User not found")
	}

	if req.Email != nil {
		email := req.Email.GetValue()
		userWithSameEmail := findUserByEmail(email)
		if userWithSameEmail != nil && userWithSameEmail.Id != user.Id {
			return nil, status.Error(codes.InvalidArgument, "User with this Email already exists")
		}
		user.Info.Email = email
	}

	if req.Name != nil {
		user.Info.Name = req.Name.GetValue()
	}

	if req.GetRole() != desc.UserRole_UNKNOWN {
		user.Info.Role = req.Role
	}

	return nil, nil
}

func findUserByEmail(email string) *desc.User {
	state.m.RLock()
	defer state.m.RUnlock()

	for _, user := range state.users {
		if user.Info.Email == email {
			return user
		}
	}
	return nil
}

func (s *server) Delete(ctx context.Context, req *desc.DeleteRequest) (*emptypb.Empty, error) {
	log.Printf("Delete, req: %+v", req)

	if req.GetId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "Id is required")
	}

	state.m.Lock()
	defer state.m.Unlock()

	_, ok := state.users[req.Id]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "User not found")
	}

	delete(state.users, req.Id)

	return nil, nil
}

func main() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	reflection.Register(s)
	desc.RegisterUserV1Server(s, &server{})

	log.Printf("server listening at %v", lis.Addr())

	if err = s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
