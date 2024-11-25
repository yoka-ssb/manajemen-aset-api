package services

import (
	"asset-management-api/assetpb"
	"context"
	"log"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type MaintenancePeriodService struct {
	MasterService
	assetpb.UnimplementedMAINTENANCEPERIODServiceServer
}

func NewMaintenancePeriodService(db *gorm.DB) *MaintenancePeriodService {
	return &MaintenancePeriodService{
		MasterService: MasterService{DB: db}}
}

func (s *MaintenancePeriodService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterMAINTENANCEPERIODServiceServer(grpcServer, s)
}

func (s *MaintenancePeriodService) ListMaintenancePeriod(ctx context.Context, req *assetpb.ListMaintenancePeriodRequest) (*assetpb.ListMaintenancePeriodResponse, error) {
	log.Default().Println("List maintenance period")
	var maintenancePeriods []*assetpb.MaintenancePeriod
	result := db.Find(&maintenancePeriods)
	if result.Error != nil {
		return &assetpb.ListMaintenancePeriodResponse{
			Data : nil,
			Message: "Error fetching data",
			Code: "500",
			}, nil
	}
	return &assetpb.ListMaintenancePeriodResponse{
		Data: maintenancePeriods,
		Message: "Success",
		Code: "200",
		}, nil
}

func (s *MaintenancePeriodService) CreateMaintenancePeriod(ctx context.Context, req *assetpb.CreateMaintenancePeriodRequest) (*assetpb.CreateMaintenancePeriodResponse, error) {
	log.Default().Println("Create maintenance period")

	maintenancePeriod := map[string]interface{}{
		"PeriodName": req.GetPeriodName(),
		"MaintenanceDate": req.GetMaintenanceDate(),
	}
	result := db.Model(&assetpb.MaintenancePeriod{}).Create(&maintenancePeriod)
	if result.Error != nil {
		return &assetpb.CreateMaintenancePeriodResponse{
			Message: "Error creating maintenance period",
			Code: "500",
			}, nil
	}

	return &assetpb.CreateMaintenancePeriodResponse{
		Message: "Success",
		Code: "200",
	}, nil
}