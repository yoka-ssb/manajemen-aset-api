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

	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type AssetService struct {
	DB *pgxpool.Pool
	assetpb.UnimplementedASSETServiceServer
}

type MstAsset struct {
	IdAssetNaming    int32
	AssetNaming      string
	ClassificationId int32
}

func NewAssetService(db *pgxpool.Pool) *AssetService {
	return &AssetService{DB: db}
}

func (s *AssetService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterASSETServiceServer(grpcServer, s)
}
func (s *AssetService) CreateAssets(ctx context.Context, req *assetpb.CreateAssetRequest) (*assetpb.CreateAssetResponse, error) {
	var createdAssets []string
	var errorsList []string
	logger := log.With().Str("service", "CreateAssets").Logger()

	// Cek apakah input kosong
	if len(req.Assets) == 0 {
		logger.Error().Msg("No asset data provided")
		return nil, status.Errorf(codes.InvalidArgument, "No asset data provided")
	}

	// Ambil AssetID terakhir dari database
	var lastAssetId int32
	err := s.DB.QueryRow(ctx, "SELECT COALESCE(MAX(asset_id), 0) FROM assets").Scan(&lastAssetId)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to retrieve last asset ID")
		return nil, status.Errorf(codes.Internal, "Failed to retrieve last asset ID")
	}

	for _, assetReq := range req.Assets {
		// Validasi Asset Classification
		var classificationEconomicValue, maintenancePeriodId int32
		err := s.DB.QueryRow(ctx, "SELECT classification_economic_value, maintenance_period_id FROM classifications WHERE classification_id = $1",
			assetReq.GetAssetClassification()).Scan(&classificationEconomicValue, &maintenancePeriodId)

		if err != nil {
			errorMsg := fmt.Sprintf("Classification not found for asset %s", assetReq.GetAssetName())
			logger.Warn().Err(err).Str("asset", assetReq.GetAssetName()).Msg(errorMsg)
			errorsList = append(errorsList, errorMsg)
			continue
		}

		// Validasi tanggal pembelian
		purchaseDate, err := time.Parse("02-01-2006", assetReq.AssetPurchaseDate)
		if err != nil {
			errorMsg := fmt.Sprintf("Invalid date format for asset: %s", assetReq.GetAssetName())
			logger.Warn().Err(err).Str("asset", assetReq.GetAssetName()).Msg(errorMsg)
			errorsList = append(errorsList, errorMsg)
			continue
		}

		// Hitung depresiasi dan nilai buku terakhir
		months := utils.CountMonths(purchaseDate, time.Now())
		if classificationEconomicValue == 0 {
			errorMsg := fmt.Sprintf("Economic value cannot be zero for asset %s", assetReq.GetAssetName())
			logger.Warn().Str("asset", assetReq.GetAssetName()).Msg(errorMsg)
			errorsList = append(errorsList, errorMsg)
			continue
		}

		deprecationValue := assetReq.GetClassificationAcquisitionValue() / classificationEconomicValue
		lastBookValue := assetReq.GetClassificationAcquisitionValue() - (deprecationValue * int32(months))

		// Hitung tanggal maintenance
		period := utils.ExtractMaintenancePeriod(maintenancePeriodId)
		maintenanceDate := time.Now().AddDate(0, period, 0)
		maintenanceDate = time.Date(maintenanceDate.Year(), maintenanceDate.Month(), 20, 0, 0, 0, 0, time.Local)
		maintenanceDateStr := maintenanceDate.Format("2006-01-02")

		// Ambil Area ID berdasarkan Outlet ID
		var areaId int32
		err = s.DB.QueryRow(ctx, "SELECT area_id FROM area_outlets WHERE outlet_id = $1", assetReq.GetOutletId()).Scan(&areaId)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to retrieve area_id for outlet %d", assetReq.GetOutletId())
			logger.Warn().Err(err).Int32("outlet_id", assetReq.GetOutletId()).Msg(errorMsg)
			errorsList = append(errorsList, errorMsg)
			continue
		}

		// Validasi id_asset_naming jika diberikan
		if assetReq.IdAssetNaming != 0 {
			var idAssetNamingExists bool
			err = s.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM mst_assets WHERE id_asset_naming = $1)", assetReq.GetIdAssetNaming()).Scan(&idAssetNamingExists)
			if err != nil || !idAssetNamingExists {
				errorMsg := fmt.Sprintf("Invalid id_asset_naming for asset %s", assetReq.GetAssetName())
				logger.Warn().Str("asset", assetReq.GetAssetName()).Msg(errorMsg)
				errorsList = append(errorsList, errorMsg)
				continue
			}
		}

		// Generate hash untuk AssetId
		assetId := lastAssetId + 1
		assetIdStr := fmt.Sprintf("%d", assetId)
		hash, err := bcrypt.GenerateFromPassword([]byte(assetIdStr), bcrypt.DefaultCost)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to generate hash for asset: %s", assetReq.GetAssetName())
			logger.Error().Err(err).Str("asset", assetReq.GetAssetName()).Msg(errorMsg)
			errorsList = append(errorsList, errorMsg)
			continue
		}

		// Insert asset ke database
		query := `
            INSERT INTO assets (
                asset_id, asset_id_hash, asset_name, asset_brand, asset_specification, 
                asset_classification, asset_condition, asset_pic, asset_purchase_date, 
                asset_maintenance_date, asset_status, classification_acquisition_value, 
                classification_last_book_value, deprecation_value, outlet_id, area_id, 
                id_asset_naming, asset_image, asset_quantity, asset_quantity_standard, personal_responsible, asset_location
            ) VALUES (
                $1, $2, $3, $4, $5, 
                $6, $7, $8, $9, $10, 
                $11, $12, $13, $14, $15, 
                $16, $17, $18, $19, $20, $21, $22
            )`
		_, err = s.DB.Exec(ctx, query,
			assetId, string(hash), assetReq.GetAssetName(), assetReq.GetAssetBrand(), assetReq.GetAssetSpecification(),
			assetReq.GetAssetClassification(), assetReq.GetAssetCondition(), assetReq.GetAssetPic(), assetReq.GetAssetPurchaseDate(),
			maintenanceDateStr, assetReq.GetAssetStatus(), assetReq.GetClassificationAcquisitionValue(),
			lastBookValue, deprecationValue, assetReq.GetOutletId(), areaId,
			assetReq.GetIdAssetNaming(), assetReq.GetAssetImage(), assetReq.GetAssetQuantity(), assetReq.GetAssetQuantityStandard(),
			assetReq.GetPersonalResponsible(), assetReq.GetAssetLocation(),
		)

		if err != nil {
			errorMsg := fmt.Sprintf("Failed to create asset: %s", assetReq.GetAssetName())
			logger.Error().Err(err).Str("asset", assetReq.GetAssetName()).Msg(errorMsg)
			errorsList = append(errorsList, errorMsg)
			continue
		}

		lastAssetId++
		logger.Info().Str("asset", assetReq.GetAssetName()).Msg("Asset created successfully")
		createdAssets = append(createdAssets, assetReq.GetAssetName())
	}

	// Jika ada error, tampilkan pesan partial success
	if len(errorsList) > 0 {
		logger.Warn().Msgf("Partial success: %d assets created, but errors occurred: %s", len(createdAssets), strings.Join(errorsList, "; "))
		return &assetpb.CreateAssetResponse{
			Message: fmt.Sprintf("Partial success: %d assets created, but errors occurred: %s", len(createdAssets), strings.Join(errorsList, "; ")),
			Code:    "206", // HTTP 206: Partial Content
			Success: len(createdAssets) > 0,
		}, nil
	}

	logger.Info().Msgf("%d assets successfully created", len(createdAssets))
	return &assetpb.CreateAssetResponse{
		Message: fmt.Sprintf("%d assets successfully created", len(createdAssets)),
		Code:    "200",
		Success: true,
	}, nil
}

func (s *AssetService) UpdateAsset(ctx context.Context, req *assetpb.UpdateAssetRequest) (*assetpb.UpdateAssetResponse, error) {
	logger := log.With().Str("service", "UpdateAsset").Logger()
	logger.Info().Int32("asset_id", req.GetId()).Msg("Updating asset")

	// Menyimpan field yang akan diupdate
	fields := []string{}
	values := []interface{}{}
	index := 1

	// Menambahkan field yang tersedia ke query
	if req.GetAssetName() != "" {
		fields = append(fields, fmt.Sprintf("asset_name = $%d", index))
		values = append(values, req.GetAssetName())
		index++
	}
	if req.GetAssetBrand() != "" {
		fields = append(fields, fmt.Sprintf("asset_brand = $%d", index))
		values = append(values, req.GetAssetBrand())
		index++
	}
	if req.GetAssetSpecification() != "" {
		fields = append(fields, fmt.Sprintf("asset_specification = $%d", index))
		values = append(values, req.GetAssetSpecification())
		index++
	}
	if req.GetAssetClassification() != 0 {
		fields = append(fields, fmt.Sprintf("asset_classification = $%d", index))
		values = append(values, req.GetAssetClassification())
		index++
	}
	if req.GetAssetCondition() != "" {
		fields = append(fields, fmt.Sprintf("asset_condition = $%d", index))
		values = append(values, req.GetAssetCondition())
		index++
	}
	if req.GetAssetPic() != 0 {
		fields = append(fields, fmt.Sprintf("asset_pic = $%d", index))
		values = append(values, req.GetAssetPic())
		index++
	}
	if req.GetAssetPurchaseDate() != "" {
		fields = append(fields, fmt.Sprintf("asset_purchase_date = $%d", index))
		values = append(values, req.GetAssetPurchaseDate())
		index++
	}
	if req.GetAssetStatus() != "" {
		fields = append(fields, fmt.Sprintf("asset_status = $%d", index))
		values = append(values, req.GetAssetStatus())
		index++
	}
	if req.GetClassificationAcquisitionValue() != 0 {
		fields = append(fields, fmt.Sprintf("classification_acquisition_value = $%d", index))
		values = append(values, req.GetClassificationAcquisitionValue())
		index++
	}
	if req.GetAssetImage() != "" {
		fields = append(fields, fmt.Sprintf("asset_image = $%d", index))
		values = append(values, req.GetAssetImage())
		index++
	}
	if req.GetPersonalResponsible() != "" {
		fields = append(fields, fmt.Sprintf("personal_responsible = $%d", index))
		values = append(values, req.GetPersonalResponsible())
		index++
	}
	if req.GetOutletId() != 0 {
		fields = append(fields, fmt.Sprintf("outlet_id = $%d", index))
		values = append(values, req.GetOutletId())
		index++
	}
	if req.GetAreaId() != 0 {
		fields = append(fields, fmt.Sprintf("area_id = $%d", index))
		values = append(values, req.GetAreaId())
		index++
	}

	// Jika tidak ada field yang diupdate, hentikan
	if len(fields) == 0 {
		logger.Warn().Int32("asset_id", req.GetId()).Msg("No fields provided for update")
		return nil, status.Error(codes.InvalidArgument, "No fields provided for update")
	}

	// Menyusun query UPDATE
	query := fmt.Sprintf("UPDATE assets SET %s WHERE asset_id = $%d", strings.Join(fields, ", "), index)
	values = append(values, req.GetId())

	// Eksekusi query
	_, err := s.DB.Exec(ctx, query, values...)
	if err != nil {
		logger.Error().Err(err).Int32("asset_id", req.GetId()).Msg("Failed to update asset")
		return nil, status.Error(codes.Internal, "Failed to update asset: "+err.Error())
	}

	// Insert data ke tabel `asset_update`
	_, err = s.DB.Exec(ctx, "INSERT INTO asset_updates (asset_id, asset_status) VALUES ($1, $2)", req.GetId(), req.GetAssetStatus())
	if err != nil {
		logger.Error().Err(err).Int32("asset_id", req.GetId()).Msg("Failed to insert asset update record")
		return nil, status.Error(codes.Internal, "Failed to insert asset update record: "+err.Error())
	}

	logger.Info().Int32("asset_id", req.GetId()).Msg("Asset successfully updated")
	return &assetpb.UpdateAssetResponse{
		Message: "Successfully updated asset",
		Code:    "200",
		Success: true,
	}, nil
}
func (s *AssetService) UpdateAssetStatus(ctx context.Context, req *assetpb.UpdateAssetStatusRequest) (*assetpb.UpdateAssetStatusResponse, error) {
	logger := log.With().Str("service", "UpdateAssetStatus").Int32("asset_id", req.GetId()).Logger()
	logger.Info().Msg("Updating asset status")

	// Get asset by id
	asset, err := s.GetAsset(ctx, &assetpb.GetAssetRequest{Id: req.GetId()})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			logger.Warn().Msg("Asset not found")
			return nil, status.Error(codes.NotFound, "Asset not found")
		}
		logger.Error().Err(err).Msg("Failed to get asset")
		return nil, status.Error(codes.Internal, "Failed to get asset")
	}

	// Getting data classification
	classification, err := s.getClassificationById(ctx, asset.Data.AssetClassification)
	if err != nil {
		logger.Warn().Msg("Classification not found")
		return nil, status.Error(codes.NotFound, "Classification not found")
	}

	// Set maintenance date
	period := utils.ExtractMaintenancePeriod(classification.MaintenancePeriodId)
	maintenanceDate := time.Now().AddDate(0, period, 0)
	maintenanceDate = time.Date(maintenanceDate.Year(), maintenanceDate.Month(), 20, 0, 0, 0, 0, time.Local)
	maintenanceDateStr := maintenanceDate.Format("2006-01-02")

	// Menyimpan field yang akan diupdate
	fields := []string{"asset_status = $1"}
	values := []interface{}{req.GetAssetStatus()}
	index := 2

	if req.GetAssetStatus() == "Baik" {
		fields = append(fields, fmt.Sprintf("asset_maintenance_date = $%d", index))
		values = append(values, maintenanceDateStr)
		index++
	}

	// Menyusun query UPDATE
	query := fmt.Sprintf("UPDATE assets SET %s WHERE asset_id = $%d", strings.Join(fields, ", "), index)
	values = append(values, req.GetId())

	// Eksekusi query
	_, err = s.DB.Exec(ctx, query, values...)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to update asset")
		return nil, status.Error(codes.Internal, "Failed to update asset: "+err.Error())
	}

	// Insert data ke tabel `asset_update`
	_, err = s.DB.Exec(ctx, "INSERT INTO asset_updates (asset_id, asset_status) VALUES ($1, $2)", req.GetId(), req.GetAssetStatus())
	if err != nil {
		logger.Error().Err(err).Msg("Failed to insert asset update record")
		return nil, status.Error(codes.Internal, "Failed to insert asset update record: "+err.Error())
	}

	logger.Info().Str("new_status", req.GetAssetStatus()).Msg("Asset status successfully updated")
	return &assetpb.UpdateAssetStatusResponse{
		Message: "Successfully updated asset status",
		Code:    "200",
		Success: true,
	}, nil
}
func (s *AssetService) getClassificationById(ctx context.Context, classificationId int32) (*assetpb.Classification, error) {
	var classification assetpb.Classification
	err := s.DB.QueryRow(ctx, "SELECT classification_id, classification_name, maintenance_period_id FROM classifications WHERE classification_id = $1", classificationId).Scan(
		&classification.ClassificationId, &classification.ClassificationName, &classification.MaintenancePeriodId)
	if err != nil {
		return nil, err
	}
	return &classification, nil
}

func (s *AssetService) ListAssetsHandler(c *gin.Context) {
	logger := log.With().Str("handler", "ListAssetsHandler").Logger()
	logger.Info().Msg("Handling asset listing request")

	// Get query parameters
	pageNumberParam := c.DefaultQuery("page_number", "1")
	pageSizeParam := c.DefaultQuery("page_size", "10")
	q := c.DefaultQuery("q", "")
	roleIDParam := c.Query("role_id")
	outletIDParam := c.Query("outlet_id")
	areaIDParam := c.Query("area_id")
	classificationParam := c.Query("classification")

	// Convert query parameters
	pageNumber, err := strconv.Atoi(pageNumberParam)
	if err != nil {
		logger.Warn().Str("page_number", pageNumberParam).Msg("Invalid page number")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}

	pageSize, err := strconv.Atoi(pageSizeParam)
	if err != nil {
		logger.Warn().Str("page_size", pageSizeParam).Msg("Invalid page size")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page size"})
		return
	}

	roleID, err := strconv.Atoi(roleIDParam)
	if err != nil {
		logger.Warn().Str("role_id", roleIDParam).Msg("Invalid role ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	var outletID *int
	if outletIDParam != "" {
		outletIDInt, err := strconv.Atoi(outletIDParam)
		if err != nil {
			logger.Warn().Str("outlet_id", outletIDParam).Msg("Invalid outlet ID")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid outlet ID"})
			return
		}
		outletID = &outletIDInt
	}

	var areaID *int
	if areaIDParam != "" {
		areaIDInt, err := strconv.Atoi(areaIDParam)
		if err != nil {
			logger.Warn().Str("area_id", areaIDParam).Msg("Invalid area ID")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid area ID"})
			return
		}
		areaID = &areaIDInt
	}

	// Build SQL query dynamically
	query := `SELECT asset_id, asset_name, asset_brand, asset_classification, asset_status, asset_condition 
			  FROM assets WHERE 1=1`
	var args []interface{}
	argIdx := 1

	// Search query (if provided)
	if q != "" {
		query += fmt.Sprintf(" AND (asset_name ILIKE $%d OR asset_brand ILIKE $%d)", argIdx, argIdx+1)
		args = append(args, "%"+q+"%", "%"+q+"%")
		argIdx += 2
	}

	// Role-based filtering
	query += fmt.Sprintf(" AND user_role_id = $%d", argIdx)
	args = append(args, roleID)
	argIdx++

	// Optional filters
	if outletID != nil {
		query += fmt.Sprintf(" AND outlet_id = $%d", argIdx)
		args = append(args, *outletID)
		argIdx++
	}

	if areaID != nil {
		query += fmt.Sprintf(" AND area_id = $%d", argIdx)
		args = append(args, *areaID)
		argIdx++
	}

	if classificationParam != "" {
		query += fmt.Sprintf(" AND asset_classification = $%d", argIdx)
		args = append(args, classificationParam)
		argIdx++
	}

	// Pagination
	query += fmt.Sprintf(" ORDER BY asset_id LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, (pageNumber-1)*pageSize)

	logger.Info().
		Str("query", query).
		Interface("args", args).
		Msg("Executing asset list query")

	// Execute query
	rows, err := s.DB.Query(context.Background(), query, args...)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch assets")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch assets: " + err.Error()})
		return
	}
	defer rows.Close()

	// Parse results
	var assets []*assetpb.Asset
	for rows.Next() {
		var asset assetpb.Asset
		if err := rows.Scan(&asset.AssetId, &asset.AssetName, &asset.AssetBrand, &asset.AssetClassification, &asset.AssetStatus, &asset.AssetCondition); err != nil {
			logger.Error().Err(err).Msg("Error scanning results")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning results: " + err.Error()})
			return
		}
		assets = append(assets, &asset)
	}

	logger.Info().Int("assets_found", len(assets)).Msg("Successfully fetched asset list")

	// Send response
	c.JSON(http.StatusOK, &assetpb.ListAssetsResponse{
		Data: assets,
	})
}
func (s *AssetService) ListAssets(ctx context.Context, req *assetpb.ListAssetsRequest) (*assetpb.ListAssetsResponse, error) {
	logger := log.With().Str("method", "ListAssets").Logger()
	logger.Info().Msg("Listing assets")

	pageNumber := req.GetPageNumber()
	pageSize := req.GetPageSize()
	q := req.GetQ()
	userRoleID := req.GetUserRoleId()
	userOutletID := req.GetUserOutletId()
	userAreaID := req.GetUserAreaId()
	classification := req.GetClassification()

	offset := (pageNumber - 1) * pageSize
	limit := pageSize

	logger.Info().
		Int32("page_number", pageNumber).
		Int32("page_size", pageSize).
		Str("query", q).
		Int32("user_role_id", userRoleID).
		Int32("user_outlet_id", userOutletID.GetValue()).
		Int32("user_area_id", userAreaID.GetValue()).
		Str("classification", classification).
		Msg("Fetching assets with filters")

	// Fetch assets
	assets, err := getAssets(s.DB, int(offset), int(limit), q, userRoleID, userOutletID, userAreaID, classification)
	if err != nil {
		logger.Error().Err(err).Msg("Error fetching assets")
		return nil, err
	}

	// Fetch total asset count
	assetTotal, err := getAssets(s.DB, 0, 0, q, userRoleID, userOutletID, userAreaID, classification)
	if err != nil {
		logger.Error().Err(err).Msg("Error fetching total count of assets")
		return nil, err
	}
	totalCount := int32(len(assetTotal))

	logger.Info().
		Int("assets_fetched", len(assets)).
		Int32("total_assets", totalCount).
		Msg("Successfully retrieved assets")

	// Construct response
	resp := &assetpb.ListAssetsResponse{
		Data:       assets,
		TotalCount: totalCount,
		PageNumber: pageNumber,
		PageSize:   pageSize,
	}

	if totalCount > offset+limit {
		resp.NextPageToken = fmt.Sprintf("page_token_%d", pageNumber+1)
	}

	logger.Info().
		Int32("next_page_number", pageNumber+1).
		Str("next_page_token", resp.NextPageToken).
		Msg("Pagination processed")

	return resp, nil
}
func (s *AssetService) GetAsset(ctx context.Context, req *assetpb.GetAssetRequest) (*assetpb.GetAssetResponse, error) {
	logger := log.With().Str("method", "GetAsset").Int32("asset_id", req.GetId()).Logger()
	logger.Info().Msg("Fetching asset by ID")

	var asset assetpb.Asset

	// Raw SQL Query
	query := `
        SELECT 
            assets.asset_id, 
            assets.asset_id_hash, 
            assets.asset_name, 
            assets.asset_brand, 
            assets.asset_specification, 
            assets.asset_classification, 
            assets.asset_status, 
            assets.asset_condition, 
            assets.asset_purchase_date, 
            assets.asset_pic, 
            assets.asset_image, 
            assets.personal_responsible, 
            assets.outlet_id, 
            assets.area_id, 
            assets.asset_maintenance_date, 
            assets.classification_acquisition_value, 
            assets.classification_last_book_value, 
            assets.created_at, 
            assets.updated_at, 
            assets.deprecation_value, 
            assets.asset_quantity, 
            assets.asset_quantity_standard, 
            assets.id_asset_naming, 
            maintenance_periods.period_name AS maintenance_period_name, 
            areas.area_name AS area_name, 
            outlets.outlet_name AS outlet_name, 
            roles.role_name AS asset_pic_name, 
            classifications.classification_name AS asset_classification_name, 
            EXTRACT(MONTH FROM AGE(CURRENT_DATE, assets.asset_purchase_date)) AS asset_age
        FROM assets
        LEFT JOIN areas ON assets.area_id = areas.area_id
        LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id
        LEFT JOIN roles ON assets.asset_pic = roles.role_id
        LEFT JOIN classifications ON assets.asset_classification = classifications.classification_id
        LEFT JOIN maintenance_periods ON classifications.maintenance_period_id = maintenance_periods.period_id
        WHERE assets.asset_id = $1
        LIMIT 1;
    `

	logger.Debug().Str("query", query).Msg("Executing SQL query")

	// Execute Query
	row := s.DB.QueryRow(ctx, query, req.GetId())

	// Scan result
	var assetIdHash, maintenancePeriodName, areaName, outletName, assetPicName, assetClassificationName sql.NullString
	var assetAge sql.NullInt64
	var idAssetNaming sql.NullInt32
	var assetPurchaseDate, assetMaintenanceDate, createdAt, updatedAt time.Time

	err := row.Scan(
		&asset.AssetId, &assetIdHash, &asset.AssetName, &asset.AssetBrand, &asset.AssetSpecification, &asset.AssetClassification,
		&asset.AssetStatus, &asset.AssetCondition, &assetPurchaseDate, &asset.AssetPic, &asset.AssetImage, &asset.PersonalResponsible,
		&asset.OutletId, &asset.AreaId, &assetMaintenanceDate, &asset.ClassificationAcquisitionValue, &asset.ClassificationLastBookValue,
		&createdAt, &updatedAt, &asset.DeprecationValue, &asset.AssetQuantity, &asset.AssetQuantityStandard, &idAssetNaming,
		&maintenancePeriodName, &areaName, &outletName, &assetPicName, &assetClassificationName, &assetAge,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn().Msg("Asset not found")
			return nil, status.Error(codes.NotFound, "Asset not found")
		}
		logger.Error().Err(err).Msg("Failed to retrieve asset")
		return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to get asset: %v", err))
	}

	asset.AssetIdHash = assetIdHash.String
	asset.MaintenancePeriodName = maintenancePeriodName.String
	asset.AreaName = areaName.String
	asset.OutletName = outletName.String
	asset.AssetPicName = assetPicName.String
	asset.AssetClassificationName = assetClassificationName.String
	asset.AssetPurchaseDate = assetPurchaseDate.Format("2006-01-02")
	asset.AssetMaintenanceDate = assetMaintenanceDate.Format("2006-01-02")
	asset.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
	asset.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")
	if assetAge.Valid {
		asset.AssetAge = int32(assetAge.Int64)
	}
	if idAssetNaming.Valid {
		asset.IdAssetNaming = idAssetNaming.Int32
	}

	logger.Info().Msg("Successfully retrieved asset")

	return &assetpb.GetAssetResponse{
		Data:    &asset,
		Code:    "200",
		Message: "Successfully retrieved asset by ID",
	}, nil
}

func getAssets(db *pgxpool.Pool, offset, limit int, q string, userRoleID int32, userOutletID, areaID *wrapperspb.Int32Value, classification string) ([]*assetpb.Asset, error) {
	logger := log.With().Str("method", "getAssets").Logger()
	logger.Info().
		Int("offset", offset).
		Int("limit", limit).
		Str("query", q).
		Int32("user_role_id", userRoleID).
		Str("classification_filter", classification).
		Msg("Fetching assets with filters")

	var assets []*assetpb.Asset

	// Base SQL Query
	query := `
        SELECT 
            assets.asset_id, 
            assets.asset_id_hash, 
            assets.asset_name, 
            assets.asset_brand, 
            assets.asset_specification, 
            assets.asset_classification, 
            assets.asset_status, 
            assets.asset_condition, 
            assets.asset_purchase_date, 
            assets.asset_pic, 
            assets.asset_image, 
            assets.personal_responsible, 
            assets.outlet_id, 
            assets.area_id, 
            assets.asset_maintenance_date, 
            assets.classification_acquisition_value, 
            assets.classification_last_book_value, 
            assets.created_at, 
            assets.updated_at, 
            assets.deprecation_value, 
            assets.asset_quantity, 
            assets.asset_quantity_standard, 
            assets.id_asset_naming, 
            maintenance_periods.period_name AS maintenance_period_name, 
            areas.area_name AS area_name, 
            outlets.outlet_name AS outlet_name, 
            roles.role_name AS asset_pic_name, 
            classifications.classification_name AS asset_classification_name, 
            EXTRACT(MONTH FROM AGE(CURRENT_DATE, assets.asset_purchase_date)) AS asset_age
        FROM assets
        LEFT JOIN areas ON assets.area_id = areas.area_id
        LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id
        LEFT JOIN roles ON assets.asset_pic = roles.role_id
        LEFT JOIN classifications ON assets.asset_classification = classifications.classification_id
        LEFT JOIN maintenance_periods ON classifications.maintenance_period_id = maintenance_periods.period_id
        WHERE 1=1 `

	// Query parameters for prepared statement
	var args []interface{}
	argIdx := 1

	// Search by asset name
	if q != "" {
		query += fmt.Sprintf(" AND assets.asset_name ILIKE $%d", argIdx)
		args = append(args, "%"+q+"%")
		argIdx++
	}

	// Filtering by role
	if userRoleID == 6 && userOutletID != nil {
		query += fmt.Sprintf(" AND assets.outlet_id = $%d", argIdx)
		args = append(args, userOutletID.GetValue())
		argIdx++
	}

	if userRoleID == 5 && areaID != nil {
		query += fmt.Sprintf(" AND assets.area_id = $%d", argIdx)
		args = append(args, areaID.GetValue())
		argIdx++
	}

	// Filtering by classification
	if classification == "perkap" {
		query += " AND assets.asset_classification = 9"
	} else {
		query += " AND assets.asset_classification <> 9"
	}

	// Apply pagination if needed
	if offset != 0 || limit != 0 {
		query += fmt.Sprintf(" ORDER BY assets.asset_name ASC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, limit, offset)
	}

	logger.Info().
		Str("query", query).
		Interface("query_params", args).
		Msg("Executing SQL query")

	// Execute Query
	rows, err := db.Query(context.Background(), query, args...)
	if err != nil {
		logger.Error().Err(err).Msg("Error executing SQL query")
		return nil, err
	}
	defer rows.Close()

	// Parse results
	for rows.Next() {
		var asset assetpb.Asset
		var assetIdHash, maintenancePeriodName, areaName, outletName, assetPicName, assetClassificationName sql.NullString
		var assetAge sql.NullInt64
		var idAssetNaming sql.NullInt32
		var assetPurchaseDate, assetMaintenanceDate, createdAt, updatedAt time.Time

		if err := rows.Scan(
			&asset.AssetId, &assetIdHash, &asset.AssetName, &asset.AssetBrand, &asset.AssetSpecification, &asset.AssetClassification,
			&asset.AssetStatus, &asset.AssetCondition, &assetPurchaseDate, &asset.AssetPic, &asset.AssetImage, &asset.PersonalResponsible,
			&asset.OutletId, &asset.AreaId, &assetMaintenanceDate, &asset.ClassificationAcquisitionValue, &asset.ClassificationLastBookValue,
			&createdAt, &updatedAt, &asset.DeprecationValue, &asset.AssetQuantity, &asset.AssetQuantityStandard, &idAssetNaming,
			&maintenancePeriodName, &areaName, &outletName, &assetPicName, &assetClassificationName, &assetAge,
		); err != nil {
			logger.Error().Err(err).Msg("Error scanning row")
			return nil, err
		}

		asset.AssetIdHash = assetIdHash.String
		asset.MaintenancePeriodName = maintenancePeriodName.String
		asset.AreaName = areaName.String
		asset.OutletName = outletName.String
		asset.AssetPicName = assetPicName.String
		asset.AssetClassificationName = assetClassificationName.String
		asset.AssetPurchaseDate = assetPurchaseDate.Format("2006-01-02")
		asset.AssetMaintenanceDate = assetMaintenanceDate.Format("2006-01-02")
		asset.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		asset.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")
		if assetAge.Valid {
			asset.AssetAge = int32(assetAge.Int64)
		}
		if idAssetNaming.Valid {
			asset.IdAssetNaming = idAssetNaming.Int32
		}

		assets = append(assets, &asset)
	}

	logger.Info().Int("assets_fetched", len(assets)).Msg("Successfully retrieved assets")
	return assets, nil
}
func (s *AssetService) GetAssetByHash(ctx context.Context, req *assetpb.GetAssetByHashRequest) (*assetpb.GetAssetByHashResponse, error) {
	logger := log.With().Str("method", "GetAssetByHash").Str("hash_id", req.GetHashId()).Logger()
	logger.Info().Msg("Fetching asset by hash ID")

	var asset assetpb.Asset

	// Raw SQL Query
	query := `
        SELECT 
            assets.asset_id, 
            assets.asset_id_hash, 
            assets.asset_name, 
            assets.asset_brand, 
            assets.asset_specification, 
            assets.asset_classification, 
            assets.asset_status, 
            assets.asset_condition, 
            assets.asset_purchase_date, 
            assets.asset_pic, 
            assets.asset_image, 
            assets.personal_responsible, 
            assets.outlet_id, 
            assets.area_id, 
            assets.asset_maintenance_date, 
            assets.classification_acquisition_value, 
            assets.classification_last_book_value, 
            assets.created_at, 
            assets.updated_at, 
            assets.deprecation_value, 
            assets.asset_quantity, 
            assets.asset_quantity_standard, 
            assets.id_asset_naming, 
            maintenance_periods.period_name AS maintenance_period_name, 
            areas.area_name AS area_name, 
            outlets.outlet_name AS outlet_name, 
            roles.role_name AS asset_pic_name, 
            classifications.classification_name AS asset_classification_name, 
            EXTRACT(MONTH FROM AGE(CURRENT_DATE, assets.asset_purchase_date)) AS asset_age
        FROM assets
        LEFT JOIN areas ON assets.area_id = areas.area_id
        LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id
        LEFT JOIN roles ON assets.asset_pic = roles.role_id
        LEFT JOIN classifications ON assets.asset_classification = classifications.classification_id
        LEFT JOIN maintenance_periods ON classifications.maintenance_period_id = maintenance_periods.period_id
        WHERE assets.asset_id_hash = $1
        LIMIT 1;
    `

	logger.Debug().Str("query", query).Msg("Executing SQL query")

	// Execute Query
	row := s.DB.QueryRow(ctx, query, req.GetHashId())

	// Scan result
	var assetIdHash, maintenancePeriodName, areaName, outletName, assetPicName, assetClassificationName sql.NullString
	var assetAge sql.NullInt64
	var idAssetNaming sql.NullInt32
	var assetPurchaseDate, assetMaintenanceDate, createdAt, updatedAt time.Time

	err := row.Scan(
		&asset.AssetId, &assetIdHash, &asset.AssetName, &asset.AssetBrand, &asset.AssetSpecification, &asset.AssetClassification,
		&asset.AssetStatus, &asset.AssetCondition, &assetPurchaseDate, &asset.AssetPic, &asset.AssetImage, &asset.PersonalResponsible,
		&asset.OutletId, &asset.AreaId, &assetMaintenanceDate, &asset.ClassificationAcquisitionValue, &asset.ClassificationLastBookValue,
		&createdAt, &updatedAt, &asset.DeprecationValue, &asset.AssetQuantity, &asset.AssetQuantityStandard, &idAssetNaming,
		&maintenancePeriodName, &areaName, &outletName, &assetPicName, &assetClassificationName, &assetAge,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn().Msg("Asset not found")
			return nil, status.Error(codes.NotFound, "Asset not found")
		}
		logger.Error().Err(err).Msg("Failed to retrieve asset")
		return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to get asset: %v", err))
	}

	asset.AssetIdHash = assetIdHash.String
	asset.MaintenancePeriodName = maintenancePeriodName.String
	asset.AreaName = areaName.String
	asset.OutletName = outletName.String
	asset.AssetPicName = assetPicName.String
	asset.AssetClassificationName = assetClassificationName.String
	asset.AssetPurchaseDate = assetPurchaseDate.Format("2006-01-02")
	asset.AssetMaintenanceDate = assetMaintenanceDate.Format("2006-01-02")
	asset.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
	asset.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")
	if assetAge.Valid {
		asset.AssetAge = int32(assetAge.Int64)
	}
	if idAssetNaming.Valid {
		asset.IdAssetNaming = idAssetNaming.Int32
	}

	logger.Info().Msg("Successfully retrieved asset by hash ID")

	return &assetpb.GetAssetByHashResponse{
		Data:    &asset,
		Code:    "200",
		Message: "Successfully retrieved asset by hash ID",
	}, nil
}

func GetAssetById(db *sql.DB, id int32) (*assetpb.Asset, error) {
	logger := log.With().Str("method", "GetAssetById").Int32("asset_id", id).Logger()
	logger.Info().Msg("Fetching asset by ID")

	var asset assetpb.Asset

	// Raw SQL Query
	query := `
		SELECT 
			assets.*, 
			maintenance_periods.period_name AS maintenance_period_name, 
			areas.area_name AS area_name, 
			outlets.outlet_name AS outlet_name, 
			roles.role_name AS asset_pic_name, 
			classifications.classification_name AS asset_classification_name, 
			EXTRACT(MONTH FROM AGE(CURRENT_DATE, assets.asset_purchase_date)) AS asset_age
		FROM assets
		LEFT JOIN areas ON assets.area_id = areas.area_id
		LEFT JOIN outlets ON assets.outlet_id = outlets.outlet_id
		LEFT JOIN roles ON assets.asset_pic = roles.role_id
		LEFT JOIN classifications ON assets.asset_classification = classifications.classification_id
		LEFT JOIN maintenance_periods ON classifications.maintenance_period_id = maintenance_periods.period_id
		WHERE assets.asset_id = $1
		LIMIT 1;
	`

	logger.Debug().Str("query", query).Msg("Executing SQL query")

	// Execute Query
	row := db.QueryRow(query, id)

	// Scan result
	var assetConditionStr sql.NullString
	var maintenancePeriodName, areaName, outletName, assetPicName, assetClassificationName sql.NullString
	var assetAge sql.NullInt64

	err := row.Scan(
		&asset.AssetId, &asset.AssetName, &asset.AssetBrand, &asset.AssetClassification,
		&asset.AssetStatus, &assetConditionStr, &asset.AssetPurchaseDate, &asset.AssetPic,
		&asset.AssetImage, &asset.PersonalResponsible, &asset.OutletId, &asset.AreaId,
		&asset.AssetMaintenanceDate, &asset.ClassificationAcquisitionValue,
		&maintenancePeriodName, &areaName, &outletName, &assetPicName, &assetClassificationName, &assetAge,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn().Msg("Asset not found")
			return nil, status.Error(codes.NotFound, "Asset not found")
		}
		logger.Error().Err(err).Msg("Failed to get asset")
		return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to get asset: %v", err))
	}

	// Set nilai tambahan ke struct
	asset.MaintenancePeriodName = maintenancePeriodName.String
	asset.AreaName = areaName.String
	asset.OutletName = outletName.String
	asset.AssetPicName = assetPicName.String
	asset.AssetClassificationName = assetClassificationName.String
	asset.AssetAge = int32(assetAge.Int64)

	// Konversi asset_condition jika diperlukan
	if assetConditionStr.Valid {
		asset.AssetCondition = assetConditionStr.String
	}

	logger.Info().Msg("Successfully retrieved asset")
	return &asset, nil
}

func GetMstAssets(ctx context.Context, db *sql.DB, offset, limit int32) ([]*MstAsset, error) {
	logger := log.With().
		Str("method", "GetMstAssets").
		Int32("offset", offset).
		Int32("limit", limit).
		Logger()

	logger.Info().Msg("Fetching master assets")

	var mstAssets []*MstAsset

	// Raw SQL Query
	query := `
		SELECT id_asset_naming, asset_naming, classification_id
		FROM mst_assets
		ORDER BY asset_naming ASC
		OFFSET $1
		LIMIT $2;
	`

	logger.Debug().Str("query", query).Msg("Executing SQL query")

	// Execute Query
	rows, err := db.QueryContext(ctx, query, offset, limit)
	if err != nil {
		logger.Error().Err(err).Msg("Error executing query")
		return nil, err
	}
	defer rows.Close()

	// Iterate through rows
	for rows.Next() {
		var asset MstAsset
		if err := rows.Scan(&asset.IdAssetNaming, &asset.AssetNaming, &asset.ClassificationId); err != nil {
			logger.Error().Err(err).Msg("Error scanning row")
			return nil, err
		}
		mstAssets = append(mstAssets, &asset)
	}

	if err := rows.Err(); err != nil {
		logger.Error().Err(err).Msg("Error iterating rows")
		return nil, err
	}

	logger.Info().Int("total_assets", len(mstAssets)).Msg("Successfully fetched master assets")

	return mstAssets, nil
}

func (s *AssetService) ListMstAssets(ctx context.Context, req *assetpb.ListMstAssetsRequest) (*assetpb.ListMstAssetsResponse, error) {
	logger := log.With().
		Str("method", "ListMstAssets").
		Int32("offset", req.Offset).
		Int32("limit", req.Limit).
		Logger()

	logger.Info().Msg("Fetching master assets")

	// Raw SQL Query
	query := `
        SELECT id_asset_naming, asset_naming, classification_id
        FROM mst_assets
        ORDER BY asset_naming ASC
        OFFSET $1
    `
	var params []interface{}
	params = append(params, req.Offset)

	// Add LIMIT clause if limit is provided and not -1
	if req.Limit > 0 {
		query += " LIMIT $2"
		params = append(params, req.Limit)
	}

	logger.Debug().Str("query", query).Msg("Executing SQL query")

	// Execute Query
	rows, err := s.DB.Query(ctx, query, params...)
	if err != nil {
		logger.Error().Err(err).Msg("Error executing query")
		return nil, err
	}
	defer rows.Close()

	var mstAssetProtos []*assetpb.MstAsset

	// Iterate through rows
	for rows.Next() {
		var asset assetpb.MstAsset
		if err := rows.Scan(&asset.IdAssetNaming, &asset.AssetNaming, &asset.ClassificationId); err != nil {
			logger.Error().Err(err).Msg("Error scanning row")
			return nil, err
		}
		mstAssetProtos = append(mstAssetProtos, &asset)
	}

	if err := rows.Err(); err != nil {
		logger.Error().Err(err).Msg("Error iterating rows")
		return nil, err
	}

	resp := &assetpb.ListMstAssetsResponse{
		Data:       mstAssetProtos,
		TotalCount: int32(len(mstAssetProtos)),
	}

	logger.Info().Int("total_assets", len(mstAssetProtos)).Msg("Successfully fetched master assets")
	return resp, nil
}
func (s *AssetService) ListMstAssetsHandler(c *gin.Context) {
	logger := log.With().Str("handler", "ListMstAssetsHandler").Logger()

	// Ambil query parameter
	offsetParam := c.DefaultQuery("offset", "0")
	limitParam := c.DefaultQuery("limit", "-1") // Set default limit to -1 (no limit)

	// Konversi ke integer
	offset, err := strconv.Atoi(offsetParam)
	if err != nil {
		logger.Error().Err(err).Str("offset", offsetParam).Msg("Invalid offset parameter")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset"})
		return
	}

	limit, err := strconv.Atoi(limitParam)
	if err != nil {
		logger.Error().Err(err).Str("limit", limitParam).Msg("Invalid limit parameter")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
		return
	}

	logger.Info().Int("offset", offset).Int("limit", limit).Msg("Fetching master assets")

	// Raw SQL Query
	query := `
        SELECT id_asset_naming, asset_naming, classification_id
        FROM mst_assets
        ORDER BY asset_naming ASC
        OFFSET $1
    `
	var params []interface{}
	params = append(params, offset)

	// Add LIMIT clause if limit is provided and not -1
	if limit > 0 {
		query += " LIMIT $2"
		params = append(params, limit)
	}

	logger.Debug().Str("query", query).Msg("Executing SQL query")

	// Eksekusi Query
	rows, err := s.DB.Query(context.Background(), query, params...)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch assets from database")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch assets"})
		return
	}
	defer rows.Close()

	var mstAssets []*assetpb.MstAsset

	// Iterasi hasil query
	for rows.Next() {
		var asset assetpb.MstAsset
		if err := rows.Scan(&asset.IdAssetNaming, &asset.AssetNaming, &asset.ClassificationId); err != nil {
			logger.Error().Err(err).Msg("Failed to scan asset row")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse assets"})
			return
		}
		mstAssets = append(mstAssets, &asset)
	}

	if err := rows.Err(); err != nil {
		logger.Error().Err(err).Msg("Error iterating asset rows")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating assets"})
		return
	}

	// Buat response
	resp := &assetpb.ListMstAssetsResponse{
		Data:       mstAssets,
		TotalCount: int32(len(mstAssets)),
	}

	logger.Info().Int("total_assets", len(mstAssets)).Msg("Successfully fetched master assets")

	// Return response
	c.JSON(http.StatusOK, resp)
}
