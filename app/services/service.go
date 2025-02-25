package services

import (
    "asset-management-api/app/database"
    "asset-management-api/assetpb"
    "github.com/rs/zerolog/log"

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

func GetTotalCount(table string) (int32, error) {
    // Query the database to get the total count of users
    var count int32
    err := db.Raw("SELECT COUNT(*) FROM " + table + "").Scan(&count).Error
    if err != nil {
        log.Error().Err(err).Msg("Error fetching total count")
        return 0, err
    }

    return count, nil
}