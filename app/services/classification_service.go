package services

import (
	"asset-management-api/assetpb"
	"context"
	"strconv"
	"strings"

	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type ClassificationService struct {
	DB *pgxpool.Pool
	assetpb.UnimplementedCLASSIFICATIONServiceServer
}

type Classification struct {
	ClassificationId            int32
	ClassificationName          string
	ClassificationEconomicValue int32
	MaintenancePeriodId         int32
	AssetHealthyParam           sql.NullString
}

func NewClassificationService(db *pgxpool.Pool) *ClassificationService {
	return &ClassificationService{DB: db}
}

func (s *ClassificationService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterCLASSIFICATIONServiceServer(grpcServer, s)
}
func (s *ClassificationService) ListClassification(ctx context.Context, req *assetpb.ListClassificationRequest) (*assetpb.ListClassificationResponse, error) {
	log.Info().Msg("Fetching classifications")

	query := `
        SELECT classification_id, classification_name, classification_economic_value, maintenance_period_id, asset_healthy_param
        FROM classifications
    `
	rows, err := s.DB.Query(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch classifications")
		return &assetpb.ListClassificationResponse{
			Data:    nil,
			Message: "Error fetching data",
			Code:    "500",
		}, nil
	}
	defer rows.Close()

	var classifications []*assetpb.Classification

	for rows.Next() {
		var classification assetpb.Classification
		var maintenancePeriodId sql.NullInt32
		var assetHealthyParam sql.NullString

		err := rows.Scan(
			&classification.ClassificationId,
			&classification.ClassificationName,
			&classification.ClassificationEconomicValue,
			&maintenancePeriodId,
			&assetHealthyParam,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan classification row")
			continue
		}

		// Convert NULL values to default values
		if maintenancePeriodId.Valid {
			classification.MaintenancePeriodId = maintenancePeriodId.Int32
		} else {
			classification.MaintenancePeriodId = 0
		}

		if assetHealthyParam.Valid {
			classification.AssetHealthyParam = assetHealthyParam.String
		} else {
			classification.AssetHealthyParam = "" // Default to empty string if NULL
		}

		classifications = append(classifications, &classification)
	}

	// Cek apakah ada error saat iterasi rows
	if err = rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating classification rows")
		return &assetpb.ListClassificationResponse{
			Data:    nil,
			Message: "Error processing data",
			Code:    "500",
		}, nil
	}

	// Jika tidak ada data ditemukan
	if len(classifications) == 0 {
		log.Warn().Msg("No classifications found")
		return &assetpb.ListClassificationResponse{
			Data:    nil,
			Message: "No classifications found",
			Code:    "404",
		}, nil
	}

	return &assetpb.ListClassificationResponse{
		Data:    classifications,
		Message: "Success",
		Code:    "200",
	}, nil
}

func (s *ClassificationService) CreateClassification(ctx context.Context, req *assetpb.CreateClassificationRequest) (*assetpb.CreateClassificationResponse, error) {
	log.Info().Msg("Creating classification")

	query := `
        INSERT INTO classifications (classification_name, classification_economic_value, maintenance_period_id, asset_healthy_param)
        VALUES ($1, $2, $3, $4)
    `
	_, err := s.DB.Exec(ctx, query,
		req.GetClassificationName(),
		req.GetClassificationEconomicValue(),
		req.GetMaintenancePeriodId(),
		req.GetAssetHealthyParam(),
	)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create classification")
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
	log.Info().Msgf("Fetching classification with ID: %d", req.GetId())

	classification := s.getClassificationById(ctx, req.GetId())
	if classification == nil {
		log.Warn().Msgf("Classification with ID %d not found", req.GetId())
		return &assetpb.GetClassificationResponse{
			Data:    nil,
			Message: "Classification not found",
			Code:    "404",
		}, nil
	}

	healthyParams := make(map[string]string)
	var splitParams []string
	if classification.AssetHealthyParam.Valid {
		splitParams = strings.Split(classification.AssetHealthyParam.String, ",")
	}

	// Parse asset healthy param
	for i, param := range splitParams {
		healthyParams["param_"+strconv.Itoa(i+1)] = param
	}

	responseClassification := &assetpb.Classification{
		ClassificationId:            classification.ClassificationId,
		ClassificationName:          classification.ClassificationName,
		ClassificationEconomicValue: classification.ClassificationEconomicValue,
		MaintenancePeriodId:         classification.MaintenancePeriodId,
		AssetHealthyParam:           classification.AssetHealthyParam.String,
		AssetHealthyParamMap:        healthyParams,
	}

	return &assetpb.GetClassificationResponse{
		Data:    responseClassification,
		Message: "Success",
		Code:    "200",
	}, nil
}
func (s *ClassificationService) getClassificationById(ctx context.Context, id int32) *Classification {
	query := `
        SELECT classification_id, classification_name, classification_economic_value, maintenance_period_id, asset_healthy_param
        FROM classifications
        WHERE classification_id = $1
        LIMIT 1
    `
	var classification Classification
	var maintenancePeriodId sql.NullInt32

	err := s.DB.QueryRow(ctx, query, id).Scan(
		&classification.ClassificationId,
		&classification.ClassificationName,
		&classification.ClassificationEconomicValue,
		&maintenancePeriodId,
		&classification.AssetHealthyParam,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Warn().Msgf("Classification with ID %d not found", id)
			return nil
		}
		log.Error().Err(err).Msg("Error fetching classification")
		return nil
	}

	// Convert NULL values to default values
	if maintenancePeriodId.Valid {
		classification.MaintenancePeriodId = maintenancePeriodId.Int32
	} else {
		classification.MaintenancePeriodId = 0
	}

	return &classification
}
