package services

import (
	"asset-management-api/app/utils"
	"asset-management-api/assetpb"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type AssetService struct {
	MasterService
	assetpb.UnimplementedASSETServiceServer
}

type Asset struct {
	AssetId                        int32  `json:"asset_id,omitempty"`
	AssetName                      string `json:"asset_name,omitempty"`
	AssetBrand                     string `json:"asset_brand,omitempty"`
	AssetSpecification             string `json:"asset_specification,omitempty"`
	AssetClassification            int32  `json:"asset_classification,omitempty"`
	AssetCondition                 string `json:"asset_condition,omitempty"`
	AssetPic                       int32  `json:"asset_pic,omitempty"`
	AssetPurchaseDate              string `json:"asset_purchase_date,omitempty"`
	AssetMaintenanceDate           string `json:"asset_maintenance_date,omitempty"`
	AssetStatus                    string `json:"asset_status,omitempty"`
	ClassificationAcquisitionValue int32  `json:"classification_acquisition_value,omitempty"`
	ClassificationLastBookValue    int32  `json:"classification_last_book_value,omitempty"`
	AssetImage                     string `json:"asset_image,omitempty"`
	PersonalResponsibleId          *int32  `json:"personal_responsible_id,omitempty"`
	DeprecationValue               int32  `json:"deprecation_value,omitempty"`
	OutletId                       int32  `json:"outlet_id,omitempty"`
	AreaId                         int32  `json:"area_id,omitempty"`
}

func NewAssetService(db *gorm.DB) *AssetService {
	return &AssetService{
		MasterService: MasterService{DB: db}}
}

func (s *AssetService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterASSETServiceServer(grpcServer, s)
}

func (s *AssetService) CreateAsset(ctx context.Context, req *assetpb.CreateAssetRequest) (*assetpb.CreateAssetResponse, error) {

	// Getting data classification
	classification := getClassificationById(req.GetAssetClassification())
	if classification == nil {
		return &assetpb.CreateAssetResponse{
			Message: "Error creating asset",
			Code:    "500",
			Success: false}, nil
	}

	// convert string to time
	purchaseDate, _ := time.Parse("2006-01-02", req.AssetPurchaseDate)

	// Count month
	month := utils.CountMonths(purchaseDate, time.Now())
	log.Default().Println("month: ", month)

	deprecationValue := req.GetClassificationAcquisitionValue() / classification.ClassificationEconomicValue

	lastBookValue := req.GetClassificationAcquisitionValue() - (deprecationValue * int32(month))
	
	// Set maintenance date
	period := utils.ExtractMaintenancePeriod(classification.MaintenancePeriodId)
	maintenanceDate := time.Now().AddDate(0, period, 0)
	maintenanceDate = time.Date(maintenanceDate.Year(), maintenanceDate.Month(), 20, 0, 0, 0, 0, time.Local)
	// Parse maintenance date to string
	maintenanceDateStr := maintenanceDate.Format("2006-01-02")
	
	var lastAsset Asset
	last := db.Model(&assetpb.Asset{}).Last(&lastAsset)
	if last.Error != nil {
		log.Println("Error:", last.Error)

		if errors.Is(last.Error, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "Asset not found")
		} else {
			return nil, status.Error(codes.Internal, "Failed to get asset: "+last.Error.Error())
		}
	}
	lastID := lastAsset.AssetId

	var personalId *int32
	if req.PersonalResponsibleId != 0 {
		personalId = &req.PersonalResponsibleId
	}

	asset := Asset{
		AssetId:                        lastID + 1,
		AssetName:                      req.GetAssetName(),
		AssetBrand:                     req.GetAssetBrand(),
		AssetSpecification:             req.GetAssetSpecification(),
		AssetClassification:            req.GetAssetClassification(),
		AssetCondition:                 req.GetAssetCondition(),
		AssetPic:                       req.GetAssetPic(),
		AssetPurchaseDate:              req.GetAssetPurchaseDate(),
		AssetMaintenanceDate:           maintenanceDateStr,
		AssetStatus:                    req.GetAssetStatus(),
		ClassificationAcquisitionValue: req.GetClassificationAcquisitionValue(),
		ClassificationLastBookValue:    lastBookValue,
		AssetImage:                     req.GetAssetImage(),
		PersonalResponsibleId:          personalId,
		DeprecationValue:               deprecationValue,
		OutletId:                       req.GetOutletId(),
		AreaId:                         req.GetAreaId(),
	}

	result := db.Create(&asset)
	if result.Error != nil {
		log.Println("Error:", result.Error)
		return nil, status.Error(codes.Internal, "Failed to create asset: "+result.Error.Error())
	}

	log.Default().Println("asset ID: ", asset.AssetId)

	// Hash asset id
	hashedAssetId, err := utils.HashPassword(fmt.Sprintf("%d", asset.AssetId))
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to hash asset id: "+err.Error())
	}

	// Update asset id hash
	update := db.Model(&assetpb.Asset{}).Where("asset_id = ?", asset.AssetId).Update("asset_id_hash", hashedAssetId)
	if update.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to update asset id hash: "+update.Error.Error())
	}

	return &assetpb.CreateAssetResponse{
		Message: "Successfully creating asset",
		Code:    "200",
		Success: true}, nil
}

func (s *AssetService) GetAsset(ctx context.Context, req *assetpb.GetAssetRequest) (*assetpb.GetAssetResponse, error) {
	log.Default().Println("getting asset with ID: ", req.GetId())
	var asset assetpb.Asset

	query := db.Select("assets.*, maintenance_periods.period_name AS maintenance_period_name, areas.area_name AS area_name, outlets.outlet_name AS outlet_name, personal_responsibles.personal_name AS personal_name, roles.role_name AS asset_pic_name, classifications.classification_name AS asset_classification_name, EXTRACT(MONTH FROM AGE(CURRENT_DATE, assets.asset_purchase_date)) AS asset_age").
		Joins("LEFT JOIN areas ON assets.area_id = areas.area_id").
		Joins("LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id").
		Joins("LEFT JOIN personal_responsibles ON assets.personal_responsible_id = personal_responsibles.personal_id").
		Joins("LEFT JOIN roles ON assets.asset_pic = roles.role_id").
		Joins("LEFT JOIN classifications ON assets.asset_classification = classifications.classification_id").
		Joins("LEFT JOIN maintenance_periods ON classifications.maintenance_period_id = maintenance_periods.period_id").
		Where("assets.asset_id = ?", req.GetId())

	result := query.First(&asset)
	if result.Error != nil {
		log.Println("Error:", result.Error)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "Asset not found")
		} else {
			return nil, status.Error(codes.Internal, "Failed to get asset")
		}
	}

	return &assetpb.GetAssetResponse{
		Data:    &asset,
		Code:    "200",
		Message: "Successfully getting asset by ID"}, nil
}

func (s *AssetService) GetAssetByHash(ctx context.Context, req *assetpb.GetAssetByHashRequest) (*assetpb.GetAssetByHashResponse, error) {
	log.Default().Println("getting asset by hash ID: ", req.GetHashId())
	var asset assetpb.Asset

	query := db.Select("assets.*, areas.area_name AS area_name, outlets.outlet_name AS outlet_name, personal_responsibles.personal_name AS personal_name, roles.role_name AS asset_pic_name, classifications.classification_name AS asset_classification_name, EXTRACT(MONTH FROM AGE(CURRENT_DATE, assets.asset_purchase_date)) AS asset_age").
		Joins("LEFT JOIN areas ON assets.area_id = areas.area_id").
		Joins("LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id").
		Joins("LEFT JOIN personal_responsibles ON assets.personal_responsible_id = personal_responsibles.personal_id").
		Joins("LEFT JOIN roles ON assets.asset_pic = roles.role_id").
		Joins("LEFT JOIN classifications ON assets.asset_classification = classifications.classification_id").
		Where("assets.asset_id_hash = ?", req.GetHashId())

	result := query.First(&asset)
	if result.Error != nil {
		log.Println("Error:", result.Error)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "Asset not found")
		} else {
			return nil, status.Error(codes.Internal, "Failed to get asset")
		}
	}
	return &assetpb.GetAssetByHashResponse{
		Data:    &asset,
		Code:    "200",
		Message: "Successfully getting asset by hash ID"}, nil
}

func (s *AssetService) UpdateAsset(ctx context.Context, req *assetpb.UpdateAssetRequest) (*assetpb.UpdateAssetResponse, error) {
	log.Default().Println("updating item")

	updates := map[string]interface{}{
		"AssetName":                       req.GetAssetName(),
		"AssetBrand":                      req.GetAssetBrand(),
		"AssetSpecification":              req.GetAssetSpecification(),
		"AssetClassification":             req.GetAssetClassification(),
		"AssetCondition":                  req.GetAssetCondition(),
		"AssetPic":                        req.GetAssetPic(),
		"AssetPurchaseDate":                req.GetAssetPurchaseDate(),
		"AssetStatus":                     req.GetAssetStatus(),
		"ClassificationAcquisitionValue": req.GetClassificationAcquisitionValue(),
		"AssetImage":                      req.GetAssetImage(),
		"PersonalResponsibleId":           req.GetPersonalResponsibleId(),
		"OutletId":                        req.GetOutletId(),
		"AreaId":                          req.GetAreaId(),
	}
	result := db.Model(&assetpb.Asset{}).Where("asset_id = ?", req.Id).Updates(updates)
	if result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to update asset: "+result.Error.Error())
	}

	// Insert data to table asset_update
	db.Create(&assetpb.AssetUpdate{
		AssetId:     req.GetId(),
		AssetStatus: req.GetAssetStatus(),
	})

	return &assetpb.UpdateAssetResponse{
		Message: "Successfully updating asset",
		Code:    "200",
		Success: true}, nil
}

func (s *AssetService) UpdateAssetStatus(ctx context.Context, req *assetpb.UpdateAssetStatusRequest) (*assetpb.UpdateAssetStatusResponse, error) {
	log.Default().Println("updating item")

	// Get asset by id
	asset, err := s.GetAsset(ctx, &assetpb.GetAssetRequest{Id: req.GetId()})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Error(codes.NotFound, "Asset not found")
		} else {
			return nil, status.Error(codes.Internal, "Failed to get asset")
		}
	}
	
	// Getting data classification
	classification := getClassificationById(asset.Data.AssetClassification)
	if classification == nil {
		return nil, status.Error(codes.NotFound, "Classification not found")
	}

	// Set maintenance date	
	period := utils.ExtractMaintenancePeriod(classification.MaintenancePeriodId)

	maintenanceDate := time.Now().AddDate(0, period, 0)
	maintenanceDate = time.Date(maintenanceDate.Year(), maintenanceDate.Month(), 20, 0, 0, 0, 0, time.Local)
	// Parse maintenance date to string
	maintenanceDateStr := maintenanceDate.Format("2006-01-02")

	var updates map[string]interface{}

	if req.GetAssetStatus() != "Baik" {
		updates = map[string]interface{}{
			"AssetStatus": req.GetAssetStatus(),
		}
	} else {
		updates = map[string]interface{}{
			"AssetStatus": req.GetAssetStatus(),
			"AssetMaintenanceDate": maintenanceDateStr,
		}
	}

	result := db.Model(&assetpb.Asset{}).Where("asset_id = ?", req.GetId()).Updates(updates)
	if result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to update asset: "+result.Error.Error())
	}

	// Insert data to table asset_update
	db.Create(&assetpb.AssetUpdate{
		AssetId:     req.GetId(),
		AssetStatus: req.GetAssetStatus(),
	})

	return &assetpb.UpdateAssetStatusResponse{
		Message: "Successfully updating asset",
		Code:    "200",
		Success: true}, nil

}

func (s *AssetService) DeleteAsset(ctx context.Context, req *assetpb.DeleteAssetRequest) (*assetpb.DeleteAssetResponse, error) {
	log.Default().Println("deleting item with ID: ", req.GetId())

	result := db.Delete(&assetpb.Asset{}, req.GetId())
	if result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to delete asset: "+result.Error.Error())
	}
	return &assetpb.DeleteAssetResponse{
		Message: "Successfully deleting asset",
		Code:    "200",
		Success: true}, nil
}

func (s *AssetService) ListAssets(ctx context.Context, req *assetpb.ListAssetsRequest) (*assetpb.ListAssetsResponse, error) {
	log.Default().Println("Listing assets")
	// Get the page number and page size from the request
	pageNumber := req.GetPageNumber()
	pageSize := req.GetPageSize()
	q := req.GetQ()

	// Calculate the offset and limit for the query
	offset := (pageNumber - 1) * pageSize
	limit := pageSize

	// Get the assets from the database
	assets, err := getAssets(offset, limit, q)
	if err != nil {
		log.Default().Println("Error fetching assets:", err)
		return nil, err
	}

	// Get the total count of assets
	totalCount, err := GetTotalCount("assets")
	if err != nil {
		log.Default().Println("Error fetching total count:", err)
		return nil, err
	}

	// Create a response
	resp := &assetpb.ListAssetsResponse{
		Data:       assets,
		TotalCount: totalCount,
		PageNumber: pageNumber,
		PageSize:   pageSize,
	}

	// Calculate the next page token
	if totalCount > offset+limit {
		resp.NextPageToken = fmt.Sprintf("page_token_%d", pageNumber+1)
	}

	return resp, nil
}

func getAssets(offset, limit int32, q string) ([]*assetpb.Asset, error) {
	// Query the database to get the assets
	var assets []*assetpb.Asset
	query := db.Select("assets.*, areas.area_name AS area_name, outlets.outlet_name AS outlet_name, personal_responsibles.personal_name AS personal_name").
		Limit(int(limit)).Offset(int(offset))

	if q != "" {
		query = query.Where("assets.asset_name LIKE ?", "%"+q+"%")
	}

	// get data area name, outlet name, personal name
	query = query.
		Joins("LEFT JOIN areas ON assets.area_id = areas.area_id").
		Joins("LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id").
		Joins("LEFT JOIN personal_responsibles ON assets.personal_responsible_id = personal_responsibles.personal_id")

	err := query.Find(&assets).Error
	if err != nil {
		log.Default().Println("Error fetching assets:", err)
		return nil, err
	}

	return assets, nil
}
