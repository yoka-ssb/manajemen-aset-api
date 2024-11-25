package services

import (
	"asset-management-api/assetpb"
	"context"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type ClassificationService struct {
	MasterService
	assetpb.UnimplementedCLASSIFICATIONServiceServer
}

func NewClassificationService(db *gorm.DB) *ClassificationService {
	return &ClassificationService{
		MasterService: MasterService{DB: db}}
}

func (s *ClassificationService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterCLASSIFICATIONServiceServer(grpcServer, s)
}

func (s *ClassificationService) ListClassification(ctx context.Context, req *assetpb.ListClassificationRequest) (*assetpb.ListClassificationResponse, error) {

	var classifications []*assetpb.Classification
	result := db.Find(&classifications)
	if result.Error != nil {
		return &assetpb.ListClassificationResponse{
		Data : nil,
		Message: "Error fetching data",
		Code: "500",
		}, nil
	}

	return &assetpb.ListClassificationResponse{
		Data: classifications,
		Message: "Success",
		Code: "200",
		}, nil
}

func (s *ClassificationService) CreateClassification(ctx context.Context, req *assetpb.CreateClassificationRequest) (*assetpb.CreateClassificationResponse, error) {
	classification := map[string]interface{}{
		"ClassificationName": req.GetClassificationName(),
		"ClassificationEconomicValue": req.GetClassificationEconomicValue(),
		"MaintenancePeriodId": req.GetMaintenancePeriodId(),
		"AssetHealthyParam": req.GetAssetHealthyParam(),
	}

	result := db.Model(&assetpb.Classification{}).Create(classification)
	if result.Error != nil {
		return &assetpb.CreateClassificationResponse{
		Message: "Error creating classification",
		Code: "500",
		}, nil
	}

	return &assetpb.CreateClassificationResponse{
		Message: "Success",
		Code: "200",
		}, nil
}