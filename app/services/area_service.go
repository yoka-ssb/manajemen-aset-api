package services

import (
	"asset-management-api/assetpb"
	"context"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type AreaService struct {
	MasterService
	assetpb.UnimplementedAREAServiceServer
}

func NewAreaService(db *gorm.DB) *AreaService {
	return &AreaService{
		MasterService: MasterService{DB: db}}
}

func (s *AreaService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterAREAServiceServer(grpcServer, s)
}

func (s *AreaService) ListArea(ctx context.Context, req *assetpb.ListAreaRequest) (*assetpb.ListAreaResponse, error) {
	var areas []*assetpb.Area
	result := db.Find(&areas)
	if result.Error != nil {
		return &assetpb.ListAreaResponse{
		Data : nil,
		Message: "Error fetching data",
		Code: "500",
		}, nil
	}
	return &assetpb.ListAreaResponse{
		Data: areas,
		Message: "Success",
		Code: "200",
		}, nil
}

func (s *AreaService) CreateArea(ctx context.Context, req *assetpb.CreateAreaRequest) (*assetpb.CreateAreaResponse, error) {

	area := map[string]interface{}{
		"AreaName": req.GetAreaName(),
	}

	result := db.Model(&assetpb.Area{}).Create(area)
	if result.Error != nil {
		return &assetpb.CreateAreaResponse{
		Message: "Error creating area",
		Code: "500",
		}, nil
	}

	return &assetpb.CreateAreaResponse{
		Message: "Success",
		Code: "200",
		}, nil
}