package services

import (
	"asset-management-api/assetpb"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type PositionService struct {
	DB *pgxpool.Pool
	assetpb.UnimplementedPOSITIONServiceServer
}

func NewPositionService(db *pgxpool.Pool) *PositionService {
	return &PositionService{DB: db}
}

func (s *PositionService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterPOSITIONServiceServer(grpcServer, s)
}

// ListArea: Mengambil daftar area menggunakan raw SQL
func (s *PositionService) ListPosition(ctx context.Context, req *assetpb.ListPositionRequest) (*assetpb.ListPositionResponse, error) {
	log.Info().Msg("Fetching list of position")
	rows, err := s.DB.Query(ctx, "SELECT id, position_name FROM positions")
	if err != nil {
		log.Error().Err(err).Msg("Error executing query")
		return &assetpb.ListPositionResponse{
			Data:    nil,
			Message: "Error fetching data",
			Code:    "500",
		}, nil
	}
	defer rows.Close()

	var positions []*assetpb.Position
	for rows.Next() {
		var position assetpb.Position
		err := rows.Scan(&position.Id, &position.PositionName)
		if err != nil {
			log.Error().Err(err).Msg("Error scanning row")
			return &assetpb.ListPositionResponse{
				Data:    nil,
				Message: "Error scanning row",
				Code:    "500",
			}, nil
		}
		positions = append(positions, &position)
	}

	if err = rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating over rows")
		return &assetpb.ListPositionResponse{
			Data:    nil,
			Message: "Error iterating over rows",
			Code:    "500",
		}, nil
	}

	log.Info().Msg("Successfully fetched list of areas")
	return &assetpb.ListPositionResponse{
		Data:    positions,
		Message: "Success",
		Code:    "200",
	}, nil
}

func (s *PositionService) CreatePosition(ctx context.Context, req *assetpb.CreatePositionRequest) (*assetpb.CreatePositionResponse, error) {
	query := `
        INSERT INTO positions (position_name) 
        VALUES ($1) 
        ON CONFLICT (position_name) 
        DO NOTHING 
        RETURNING position_id`

	var positionId int32
	err := s.DB.QueryRow(ctx, query, req.GetPositionName()).Scan(&positionId)

	if err != nil {
		log.Error().Err(err).Msg("Error creating position")
		return &assetpb.CreatePositionResponse{
			Message: "Error creating position: " + err.Error(),
			Code:    "500",
			Success: false,
		}, nil
	}

	log.Info().Msgf("Successfully created position with ID: %d", positionId)
	return &assetpb.CreatePositionResponse{
		Message: "Success",
		Code:    "200",
		Success: true,
	}, nil
}
