package services

import (
    "asset-management-api/assetpb"
    "context"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "gorm.io/gorm"
)

type RoleService struct {
    MasterService
    assetpb.UnimplementedROLEServiceServer
}

func NewRoleService(db *gorm.DB) *RoleService {
    return &RoleService{
        MasterService: MasterService{DB: db}}
}

func (s *RoleService) Register(server interface{}) {
    grpcServer := server.(grpc.ServiceRegistrar)
    assetpb.RegisterROLEServiceServer(grpcServer, s)
}

func (s *RoleService) ListRole(ctx context.Context, req *assetpb.ListRoleRequest) (*assetpb.ListRoleResponse, error) {
    log.Info().Msg("List role")
    
    var roles []*assetpb.Role
    result := db.Find(&roles)
    if result.Error != nil {
        log.Error().Err(result.Error).Msg("Error fetching data")
        return &assetpb.ListRoleResponse{
            Data: nil,
            Message: "Error fetching data",
            Code: "500",
        }, nil
    }
    return &assetpb.ListRoleResponse{
        Data: roles,
        Message: "Success",
        Code: "200",
    }, nil
}