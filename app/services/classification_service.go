package services

import (
	"asset-management-api/assetpb"
	"context"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type ClassificationService struct {
	MasterService
	assetpb.UnimplementedCLASSIFICATIONServiceServer
}

type Classification struct {
	ClassificationId            int32
	ClassificationName          string
	ClassificationEconomicValue int32
	MaintenancePeriodId         int32
	AssetHealthyParam           string
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

	var classifications []*Classification
	result := db.Find(&classifications)
	if result.Error != nil {
		return &assetpb.ListClassificationResponse{
			Data:    nil,
			Message: "Error fetching data",
			Code:    "500",
		}, nil
	}

	// Convert classifications to []*assetpb.Classification
	var dataClassifications []*assetpb.Classification
	for _, classification := range classifications {
		dataClassification := &assetpb.Classification{
			ClassificationId:            classification.ClassificationId,
			ClassificationName:          classification.ClassificationName,
			ClassificationEconomicValue: classification.ClassificationEconomicValue,
			MaintenancePeriodId:         classification.MaintenancePeriodId,
			AssetHealthyParam:           classification.AssetHealthyParam,
		}
		dataClassifications = append(dataClassifications, dataClassification)
	}

	return &assetpb.ListClassificationResponse{
		Data:    dataClassifications,
		Message: "Success",
		Code:    "200",
	}, nil
}

func (s *ClassificationService) CreateClassification(ctx context.Context, req *assetpb.CreateClassificationRequest) (*assetpb.CreateClassificationResponse, error) {
	classification := map[string]interface{}{
		"ClassificationName":          req.GetClassificationName(),
		"ClassificationEconomicValue": req.GetClassificationEconomicValue(),
		"MaintenancePeriodId":         req.GetMaintenancePeriodId(),
		"AssetHealthyParam":           req.GetAssetHealthyParam(),
	}

	result := db.Model(&assetpb.Classification{}).Create(classification)
	if result.Error != nil {
		return &assetpb.CreateClassificationResponse{
			Message: "Error creating classification",
			Code:    "500",
		}, nil
	}

	return &assetpb.CreateClassificationResponse{
		Message: "Success",
		Code:    "200",
	}, nil
}

func (s *ClassificationService) GetClassification(ctx context.Context, req *assetpb.GetClassificationRequest) (*assetpb.GetClassificationResponse, error) {
	getClassification := getClassificationById(req.GetId())

	healthyParams := make(map[string]string)
	splitParams := strings.Split(getClassification.AssetHealthyParam, ",")

	// Parse asset healty param
	for i, param := range splitParams {
		str := strconv.Itoa(i + 1)
		healthyParams["param_"+str] = param
	}

	classification := &assetpb.Classification{
		ClassificationId:            getClassification.ClassificationId,
		ClassificationName:          getClassification.ClassificationName,
		ClassificationEconomicValue: getClassification.ClassificationEconomicValue,
		MaintenancePeriodId:         getClassification.MaintenancePeriodId,
		AssetHealthyParam:           getClassification.AssetHealthyParam,
		AssetHealthyParamMap:        healthyParams,
	}

	return &assetpb.GetClassificationResponse{
		Data:    classification,
		Message: "Success",
		Code:    "200",
	}, nil
}

func getClassificationById(id int32) *Classification {

	var classification Classification
	db.First(&classification, id)
	return &classification
}
