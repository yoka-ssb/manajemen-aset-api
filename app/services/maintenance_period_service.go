package services

import (
	"asset-management-api/assetpb"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type MaintenancePeriodService struct {
	DB *pgxpool.Pool
	assetpb.UnimplementedMAINTENANCEPERIODServiceServer
}

func NewMaintenancePeriodService(db *pgxpool.Pool) *MaintenancePeriodService {
	return &MaintenancePeriodService{DB: db}
}

func (s *MaintenancePeriodService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterMAINTENANCEPERIODServiceServer(grpcServer, s)
}

func (s *MaintenancePeriodService) ListMaintenancePeriod(ctx context.Context, req *assetpb.ListMaintenancePeriodRequest) (*assetpb.ListMaintenancePeriodResponse, error) {
	log.Info().Msg("Fetching maintenance periods")

	query := `
        SELECT id, period_name, maintenance_date
        FROM maintenance_periods
    `
	rows, err := s.DB.Query(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch maintenance periods")
		return &assetpb.ListMaintenancePeriodResponse{
			Data:    nil,
			Message: "Error fetching data",
			Code:    "500",
		}, nil
	}
	defer rows.Close()

	var maintenancePeriods []*assetpb.MaintenancePeriod

	for rows.Next() {
		var period assetpb.MaintenancePeriod
		err := rows.Scan(
			&period.PeriodId,
			&period.PeriodName,
			&period.MaintenanceDate,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan maintenance period row")
			continue
		}
		maintenancePeriods = append(maintenancePeriods, &period)
	}

	if err = rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating maintenance period rows")
	}

	return &assetpb.ListMaintenancePeriodResponse{
		Data:    maintenancePeriods,
		Message: "Success",
		Code:    "200",
	}, nil
}

func (s *MaintenancePeriodService) CreateMaintenancePeriod(ctx context.Context, req *assetpb.CreateMaintenancePeriodRequest) (*assetpb.CreateMaintenancePeriodResponse, error) {
	log.Info().Msg("Creating maintenance period")

	query := `
        INSERT INTO maintenance_periods (period_name, maintenance_date)
        VALUES ($1, $2)
    `
	_, err := s.DB.Exec(ctx, query, req.GetPeriodName(), req.GetMaintenanceDate())

	if err != nil {
		log.Error().Err(err).Msg("Failed to create maintenance period")
		return &assetpb.CreateMaintenancePeriodResponse{
			Message: "Error creating maintenance period",
			Code:    "500",
		}, nil
	}

	return &assetpb.CreateMaintenancePeriodResponse{
		Message: "Success",
		Code:    "200",
	}, nil
}
