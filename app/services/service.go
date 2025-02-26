package services

import (
	"asset-management-api/app/database"
	"asset-management-api/assetpb"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

var db *pgxpool.Pool

func init() {
	db = database.DBConn()
}

// MasterService contains shared methods and attributes for all services.
type MasterService struct {
	DB *pgxpool.Pool
	assetpb.ASSETServiceServer
}

func NewService(db *pgxpool.Pool) *MasterService {
	return &MasterService{
		DB: db,
	}
}

func (s *MasterService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterASSETServiceServer(grpcServer, s)
}

// InterfaceService provides a common interface for all services.
type InterfaceService interface {
	Register(server interface{})
}

func GetTotalCount(table string) (int32, error) {
	var count int32
	query := "SELECT COUNT(*) FROM " + table
	err := db.QueryRow(context.Background(), query).Scan(&count)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching total count")
		return 0, err
	}

	return count, nil
}
