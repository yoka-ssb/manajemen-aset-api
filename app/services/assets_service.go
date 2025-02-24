package services

import (
	"asset-management-api/app/utils"
	"asset-management-api/assetpb"
	"context"
	"errors"
	"fmt"

	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/gorm"
)

type AssetService struct {
	MasterService
	assetpb.UnimplementedASSETServiceServer
}

type MstAssetService struct {
	DB *gorm.DB
}

type MstAsset struct {
	AssetNaming      string `json:"asset_naming"`
	ClassificationId int32  `json:"classification_id"`
}

type AreaOutlet struct {
	OutletId int32 `json:"outlet_id"`
	AreaId   int32 `json:"area_id"`
}

type Asset struct {
	AssetIdHash                    string  `json:"asset_id_hash,omitempty"`
	AssetId                        int32   `json:"asset_id,omitempty"`
	AssetName                      string  `json:"asset_name,omitempty"`
	AssetBrand                     string  `json:"asset_brand,omitempty"`
	AssetSpecification             string  `json:"asset_specification,omitempty"`
	AssetClassification            int32   `json:"asset_classification,omitempty"`
	AssetCondition                 string  `json:"asset_condition,omitempty"`
	AssetPic                       int32   `json:"asset_pic,omitempty"`
	AssetPurchaseDate              string  `json:"asset_purchase_date,omitempty"`
	AssetMaintenanceDate           string  `json:"asset_maintenance_date,omitempty"`
	AssetStatus                    string  `json:"asset_status,omitempty"`
	ClassificationAcquisitionValue int32   `json:"classification_acquisition_value,omitempty"`
	ClassificationLastBookValue    int32   `json:"classification_last_book_value,omitempty"`
	AssetImage                     string  `json:"asset_image,omitempty"`
	PersonalResponsible            *string `json:"personal_responsible,omitempty"`
	DeprecationValue               int32   `json:"deprecation_value,omitempty"`
	OutletId                       int32   `json:"outlet_id,omitempty"`
	AreaId                         int32   `json:"area_id,omitempty"`
	IdAssetNaming                  int32   `json:"id_asset_naming,omitempty"`
	AssetQuantity                  int32   `json:"asset_quantity,omitempty"`
	AssetQuantityStandard          int32   `json:"asset_quantity_standard,omitempty"`
}

func NewAssetService(db *gorm.DB) *AssetService {
	return &AssetService{
		MasterService: MasterService{DB: db}}
}

func (s *AssetService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterASSETServiceServer(grpcServer, s)
}
func (s *AssetService) CreateAssets(ctx context.Context, req *assetpb.CreateAssetRequest) (*assetpb.CreateAssetResponse, error) {
	var createdAssets []Asset
	var errorsList []string

	// Cek apakah input kosong
	if len(req.Assets) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "No asset data provided")
	}

	// Cek apakah input adalah single asset (jika ya, ubah ke array)
	if len(req.Assets) == 1 && req.Assets[0].GetAssetName() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Asset name cannot be empty")
	}

	var lastAssetId int32
	db.Model(&Asset{}).Select("COALESCE(MAX(asset_id), 0)").Scan(&lastAssetId)

	for _, assetReq := range req.Assets {
		classification := getClassificationById(assetReq.GetAssetClassification())
		if classification == nil {
			errorsList = append(errorsList, fmt.Sprintf("Error: Classification not found for asset %s", assetReq.GetAssetName()))
			continue
		}

		purchaseDate, err := time.Parse("02-01-2006", assetReq.AssetPurchaseDate)
		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("Invalid date format for asset: %s", assetReq.GetAssetName()))
			continue
		}

		month := utils.CountMonths(purchaseDate, time.Now())
		if classification.ClassificationEconomicValue == 0 {
			errorsList = append(errorsList, fmt.Sprintf("Economic value cannot be zero for asset %s", assetReq.GetAssetName()))
			continue
		}

		deprecationValue := assetReq.GetClassificationAcquisitionValue() / classification.ClassificationEconomicValue
		lastBookValue := assetReq.GetClassificationAcquisitionValue() - (deprecationValue * int32(month))

		period := utils.ExtractMaintenancePeriod(classification.MaintenancePeriodId)
		maintenanceDate := time.Now().AddDate(0, period, 0)
		maintenanceDate = time.Date(maintenanceDate.Year(), maintenanceDate.Month(), 20, 0, 0, 0, 0, time.Local)
		maintenanceDateStr := maintenanceDate.Format("2006-01-02")

		var areaOutlet AreaOutlet
		if err := db.Where("outlet_id = ?", assetReq.GetOutletId()).First(&areaOutlet).Error; err != nil {
			errorsList = append(errorsList, fmt.Sprintf("Failed to retrieve area_id for outlet %d", assetReq.GetOutletId()))
			continue
		}

		personalResponsible := assetReq.PersonalResponsible

		newAsset := Asset{
			AssetId:                        lastAssetId + 1,
			AssetName:                      assetReq.GetAssetName(),
			AssetBrand:                     assetReq.GetAssetBrand(),
			AssetSpecification:             assetReq.GetAssetSpecification(),
			AssetClassification:            assetReq.GetAssetClassification(),
			AssetCondition:                 assetReq.GetAssetCondition(),
			AssetPic:                       assetReq.GetAssetPic(),
			AssetPurchaseDate:              assetReq.GetAssetPurchaseDate(),
			AssetMaintenanceDate:           maintenanceDateStr,
			AssetStatus:                    assetReq.GetAssetStatus(),
			ClassificationAcquisitionValue: assetReq.GetClassificationAcquisitionValue(),
			ClassificationLastBookValue:    lastBookValue,
			PersonalResponsible:            &personalResponsible,
			DeprecationValue:               deprecationValue,
			OutletId:                       assetReq.GetOutletId(),
			AreaId:                         areaOutlet.AreaId,
			AssetImage:                     assetReq.GetAssetImage(),
			AssetQuantityStandard:          assetReq.GetAssetQuantityStandard(),
			AssetQuantity:                  assetReq.GetAssetQuantity(),
		}

		// Generate hash for AssetId
		assetIdStr := fmt.Sprintf("%d", newAsset.AssetId)
		hash, err := bcrypt.GenerateFromPassword([]byte(assetIdStr), bcrypt.DefaultCost)
		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("Failed to generate hash for asset: %s", newAsset.AssetName))
			continue
		}
		newAsset.AssetIdHash = string(hash)

		if err := db.Create(&newAsset).Error; err != nil {
			errorsList = append(errorsList, fmt.Sprintf("Failed to create asset: %s", newAsset.AssetName))
			continue
		}

		lastAssetId++
		createdAssets = append(createdAssets, newAsset)
	}

	// Jika ada error, tampilkan pesan partial success
	if len(errorsList) > 0 {
		return &assetpb.CreateAssetResponse{
			Message: fmt.Sprintf("Partial success: %d assets created, but errors occurred: %s", len(createdAssets), strings.Join(errorsList, "; ")),
			Code:    "206", // HTTP 206: Partial Content
			Success: len(createdAssets) > 0,
		}, nil
	}

	return &assetpb.CreateAssetResponse{
		Message: fmt.Sprintf("%d assets successfully created", len(createdAssets)),
		Code:    "200",
		Success: true,
	}, nil
}

func (s *AssetService) DeleteAsset(ctx context.Context, req *assetpb.DeleteAssetRequest) (*assetpb.DeleteAssetResponse, error) {
	log.Info().Msgf("deleting item with ID: %d", req.GetId())

	result := db.Delete(&assetpb.Asset{}, req.GetId())
	if result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to delete asset: "+result.Error.Error())
	}
	return &assetpb.DeleteAssetResponse{
		Message: "Successfully deleting asset",
		Code:    "200",
		Success: true}, nil
}

func (s *AssetService) UpdateAsset(ctx context.Context, req *assetpb.UpdateAssetRequest) (*assetpb.UpdateAssetResponse, error) {
	log.Info().Msg("updating item")

	updates := map[string]interface{}{
		"AssetName":                      req.GetAssetName(),
		"AssetBrand":                     req.GetAssetBrand(),
		"AssetSpecification":             req.GetAssetSpecification(),
		"AssetClassification":            req.GetAssetClassification(),
		"AssetCondition":                 req.GetAssetCondition(),
		"AssetPic":                       req.GetAssetPic(),
		"AssetPurchaseDate":              req.GetAssetPurchaseDate(),
		"AssetStatus":                    req.GetAssetStatus(),
		"ClassificationAcquisitionValue": req.GetClassificationAcquisitionValue(),
		"AssetImage":                     req.GetAssetImage(),
		"PersonalResponsible":            req.GetPersonalResponsible(),
		"OutletId":                       req.GetOutletId(),
		"AreaId":                         req.GetAreaId(),
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
	log.Info().Msg("updating item")

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
			"AssetStatus":          req.GetAssetStatus(),
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

func (s *AssetService) ListAssetsHandler(c *gin.Context) {
	// Get query parameters
	pageNumberParam := c.DefaultQuery("page_number", "1")
	pageSizeParam := c.DefaultQuery("page_size", "10")
	q := c.DefaultQuery("q", "")
	roleIDParam := c.Query("role_id")
	outletIDParam := c.Query("outlet_id")
	areaIDParam := c.Query("area_id")
	classificationParam := c.Query("classification")

	// Convert query parameters to appropriate types
	pageNumber, err := strconv.Atoi(pageNumberParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}

	pageSize, err := strconv.Atoi(pageSizeParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page size"})
		return
	}

	roleID, err := strconv.Atoi(roleIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	var outletID *wrapperspb.Int32Value
	if outletIDParam != "" {
		outletIDInt, err := strconv.Atoi(outletIDParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid outlet ID"})
			return
		}
		outletID = wrapperspb.Int32(int32(outletIDInt))
	}

	var areaID *wrapperspb.Int32Value
	if areaIDParam != "" {
		areaIDInt, err := strconv.Atoi(areaIDParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid area ID"})
			return
		}
		areaID = wrapperspb.Int32(int32(areaIDInt))
	}

	// Create the request object
	req := &assetpb.ListAssetsRequest{
		PageNumber:     int32(pageNumber),
		PageSize:       int32(pageSize),
		Q:              q,
		UserRoleId:     int32(roleID),
		UserOutletId:   outletID,
		UserAreaId:     areaID,
		Classification: classificationParam,
	}

	// Call the ListAssets function
	resp, err := s.ListAssets(context.Background(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return the response
	c.JSON(http.StatusOK, resp)
}
func (s *AssetService) ListAssets(ctx context.Context, req *assetpb.ListAssetsRequest) (*assetpb.ListAssetsResponse, error) {
	log.Info().Msg("Listing assets")

	pageNumber := req.GetPageNumber()
	pageSize := req.GetPageSize()
	q := req.GetQ()
	userRoleID := req.GetUserRoleId()
	userOutletID := req.GetUserOutletId()
	userAreaID := req.GetUserAreaId()
	classification := req.GetClassification()

	offset := (pageNumber - 1) * pageSize
	limit := pageSize

	assets, err := getAssets(int(offset), int(limit), q, userRoleID, userOutletID, userAreaID, classification)
	if err != nil {
		log.Info().Err(err).Msg("Error fetching assets")
		return nil, err
	}

	assetTotal, err := getAssets(0, 0, q, userRoleID, userOutletID, userAreaID, classification)
	if err != nil {
		log.Error().Msg("Error fetching total count of assets: " + err.Error())
		return nil, err
	}
	totalCount := int32(len(assetTotal))

	resp := &assetpb.ListAssetsResponse{
		Data:       assets,
		TotalCount: totalCount,
		PageNumber: pageNumber,
		PageSize:   pageSize,
	}

	if totalCount > offset+limit {
		resp.NextPageToken = fmt.Sprintf("page_token_%d", pageNumber+1)
	}

	return resp, nil
}

func getAssets(offset, limit int, q string, userRoleID int32, userOutletID, areaID *wrapperspb.Int32Value, classification string) ([]*assetpb.Asset, error) {
	var assets []*assetpb.Asset
	var query *gorm.DB

	// For counting, no pagination, only filters applied
	if offset == 0 && limit == 0 {
		query = db.Select("assets.*, maintenance_periods.period_name AS maintenance_period_name, areas.area_name AS area_name, outlets.outlet_name AS outlet_name, roles.role_name AS asset_pic_name, classifications.classification_name AS asset_classification_name, EXTRACT(MONTH FROM AGE(CURRENT_DATE, assets.asset_purchase_date)) AS asset_age").
			Joins("LEFT JOIN areas ON assets.area_id = areas.area_id").
			Joins("LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id").
			Joins("LEFT JOIN roles ON assets.asset_pic = roles.role_id").
			Joins("LEFT JOIN classifications ON assets.asset_classification = classifications.classification_id").
			Joins("LEFT JOIN maintenance_periods ON classifications.maintenance_period_id = maintenance_periods.period_id").
			Order("assets.asset_name ASC") // Order by asset_name in ascending order
	} else {
		// For fetching paginated results
		query = db.Select("assets.*, maintenance_periods.period_name AS maintenance_period_name, areas.area_name AS area_name, outlets.outlet_name AS outlet_name, roles.role_name AS asset_pic_name, classifications.classification_name AS asset_classification_name, EXTRACT(MONTH FROM AGE(CURRENT_DATE, assets.asset_purchase_date)) AS asset_age").
			Joins("LEFT JOIN areas ON assets.area_id = areas.area_id").
			Joins("LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id").
			Joins("LEFT JOIN roles ON assets.asset_pic = roles.role_id").
			Joins("LEFT JOIN classifications ON assets.asset_classification = classifications.classification_id").
			Joins("LEFT JOIN maintenance_periods ON classifications.maintenance_period_id = maintenance_periods.period_id").
			Offset(offset).Limit(limit).
			Order("assets.asset_name ASC") // Order by asset_name in ascending order
	}
	if q != "" {
		query = query.Where("assets.asset_name LIKE ?", "%"+q+"%")
	}
	if userRoleID == 6 && userOutletID != nil {
		query = query.Where("assets.outlet_id = ?", userOutletID.GetValue())
	}
	if userRoleID == 5 && areaID != nil {
		query = query.Where("assets.area_id = ?", areaID.GetValue())
	}
	if classification == "perkap" {
		query = query.Where("assets.asset_classification = 9")
	} else {
		query = query.Where("assets.asset_classification <> 9")
	}
	result := query.Find(&assets)
	if result.Error != nil {
		return nil, result.Error
	}

	return assets, nil
}

func GetAssetById(id int32) (*assetpb.Asset, error) {
	var asset assetpb.Asset
	query := db.Select("assets.*, maintenance_periods.period_name AS maintenance_period_name, areas.area_name AS area_name, outlets.outlet_name AS outlet_name, roles.role_name AS asset_pic_name, classifications.classification_name AS asset_classification_name, EXTRACT(MONTH FROM AGE(CURRENT_DATE, assets.asset_purchase_date)) AS asset_age").
		Joins("LEFT JOIN areas ON assets.area_id = areas.area_id").
		Joins("LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id").
		Joins("LEFT JOIN roles ON assets.asset_pic = roles.role_id").
		Joins("LEFT JOIN classifications ON assets.asset_classification = classifications.classification_id").
		Joins("LEFT JOIN maintenance_periods ON classifications.maintenance_period_id = maintenance_periods.period_id").
		Where("assets.asset_id = ?", id)

	result := query.First(&asset)
	if result.Error != nil {
		log.Error().Msg("Error: " + result.Error.Error())
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "Asset not found")
		} else {
			return nil, status.Error(codes.Internal, "Failed to get asset")
		}
	}
	return &asset, nil
}

func (s *AssetService) GetAsset(ctx context.Context, req *assetpb.GetAssetRequest) (*assetpb.GetAssetResponse, error) {
	log.Info().Msgf("getting asset with ID: %d", req.GetId())
	var asset assetpb.Asset

	query := db.Select("assets.*, maintenance_periods.period_name AS maintenance_period_name, areas.area_name AS area_name, outlets.outlet_name AS outlet_name, roles.role_name AS asset_pic_name, classifications.classification_name AS asset_classification_name, EXTRACT(MONTH FROM AGE(CURRENT_DATE, assets.asset_purchase_date)) AS asset_age").
		Joins("LEFT JOIN areas ON assets.area_id = areas.area_id").
		Joins("LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id").
		Joins("LEFT JOIN roles ON assets.asset_pic = roles.role_id").
		Joins("LEFT JOIN classifications ON assets.asset_classification = classifications.classification_id").
		Joins("LEFT JOIN maintenance_periods ON classifications.maintenance_period_id = maintenance_periods.period_id").
		Where("assets.asset_id = ?", req.GetId())

	result := query.First(&asset)
	if result.Error != nil {
		log.Error().Msg("Error: " + result.Error.Error())
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
	log.Info().Msg("getting asset by hash ID: " + req.GetHashId())
	var asset assetpb.Asset

	query := db.Select("assets.*, areas.area_name AS area_name, outlets.outlet_name AS outlet_name, roles.role_name AS asset_pic_name, classifications.classification_name AS asset_classification_name, EXTRACT(MONTH FROM AGE(CURRENT_DATE, assets.asset_purchase_date)) AS asset_age").
		Joins("LEFT JOIN areas ON assets.area_id = areas.area_id").
		Joins("LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id").
		Joins("LEFT JOIN roles ON assets.asset_pic = roles.role_id").
		Joins("LEFT JOIN classifications ON assets.asset_classification = classifications.classification_id").
		Where("assets.asset_id_hash = ?", req.GetHashId())

	result := query.First(&asset)
	if result.Error != nil {
		log.Error().Msg("Error: " + result.Error.Error())
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

func GetMstAssets(db *gorm.DB, offset, limit int32) ([]*MstAsset, error) {
	var mstAssets []*MstAsset
	query := db.Table("mst_assets").
		Select("asset_naming, classification_id").
		Offset(int(offset)).
		Order("asset_naming ASC")

	if limit > 0 {
		query = query.Limit(int(limit))
	}

	log.Info().Msgf("Executing query with offset: %d, limit: %d", offset, limit)

	result := query.Find(&mstAssets)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Error executing query")
		return nil, result.Error
	}

	log.Info().Msgf("Query executed successfully, found %d assets", len(mstAssets))

	return mstAssets, nil
}

func (s *AssetService) ListMstAssets(ctx context.Context, req *assetpb.ListMstAssetsRequest) (*assetpb.ListMstAssetsResponse, error) {
	mstAssets, err := GetMstAssets(s.DB, req.Offset, req.Limit)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching assets")
		return nil, err
	}

	var mstAssetProtos []*assetpb.MstAsset
	for _, asset := range mstAssets {
		mstAssetProtos = append(mstAssetProtos, &assetpb.MstAsset{
			AssetNaming:      asset.AssetNaming,
			ClassificationId: asset.ClassificationId,
		})
	}

	resp := &assetpb.ListMstAssetsResponse{
		Data:       mstAssetProtos,
		TotalCount: int32(len(mstAssets)),
	}

	return resp, nil
}

func (s *AssetService) ListMstAssetsHandler(c *gin.Context) {
	offsetParam := c.DefaultQuery("offset", "0")
	limitParam := c.DefaultQuery("limit", "0")

	offset, err := strconv.Atoi(offsetParam)
	if err != nil {
		log.Error().Err(err).Msg("Invalid offset")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset"})
		return
	}

	limit, err := strconv.Atoi(limitParam)
	if err != nil {
		log.Error().Err(err).Msg("Invalid limit")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
		return
	}

	req := &assetpb.ListMstAssetsRequest{
		Offset: int32(offset),
		Limit:  int32(limit),
	}

	resp, err := s.ListMstAssets(context.Background(), req)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching assets")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
