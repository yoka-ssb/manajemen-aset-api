package services

import (
	"asset-management-api/app/database"
	"asset-management-api/assetpb"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

var db = database.DBConn()

// MasterService contains shared methods and attributes for all services.
type MasterService struct {
	DB *gorm.DB
	assetpb.ASSETServiceServer
}

func NewService(db *gorm.DB) *MasterService {
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