package services

import (
	"asset-management-api/assetpb"
	"context"
	"log"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type PersonalResponsibleService struct {
	MasterService
	assetpb.UnimplementedPERSONALRESPONSIBLEServiceServer
}

func NewPersonalResponsibleService(db *gorm.DB) *PersonalResponsibleService {
	return &PersonalResponsibleService{
		MasterService: MasterService{DB: db}}
}

func (s *PersonalResponsibleService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterPERSONALRESPONSIBLEServiceServer(grpcServer, s)
}

func (s *PersonalResponsibleService) ListPersonalResponsible(ctx context.Context, req *assetpb.ListPersonalResponsibleRequest) (*assetpb.ListPersonalResponsibleResponse, error) {
	log.Default().Println("getting all personal responsibles")

	var personalResponsible []*assetpb.PersonalResponsible
	result := db.Find(&personalResponsible)

	if result.Error != nil {
		return &assetpb.ListPersonalResponsibleResponse{
			Data : nil,
			Message: "Error fetching data",
			Code: "500",
			}, nil
	}
	return &assetpb.ListPersonalResponsibleResponse{
		Data: personalResponsible,
		Message: "Success",
		Code: "200",
		}, nil
}