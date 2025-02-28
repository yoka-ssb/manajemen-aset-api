package services

import (
	"asset-management-api/assetpb"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type RoleService struct {
	MasterService
	assetpb.UnimplementedROLEServiceServer
	DB *pgxpool.Pool
}

func NewRoleService(db *pgxpool.Pool) *RoleService {
	return &RoleService{
		MasterService: MasterService{},
		DB:            db,
	}
}

func (s *RoleService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterROLEServiceServer(grpcServer, s)
}

func (s *RoleService) ListRole(ctx context.Context, req *assetpb.ListRoleRequest) (*assetpb.ListRoleResponse, error) {
	log.Info().Msg("List role")

	query := "SELECT role_id, role_name, status FROM roles"
	rows, err := s.DB.Query(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching data")
		return &assetpb.ListRoleResponse{
			Data:    nil,
			Message: "Error fetching data",
			Code:    "500",
		}, nil
	}
	defer rows.Close()

	var roles []*assetpb.Role
	for rows.Next() {
		var role assetpb.Role
		if err := rows.Scan(&role.RoleId, &role.RoleName, &role.Status); err != nil {
			log.Error().Err(err).Msg("Error scanning row")
			continue
		}
		roles = append(roles, &role)
	}

	return &assetpb.ListRoleResponse{
		Data:    roles,
		Message: "Success",
		Code:    "200",
	}, nil
}
