package services

import (
	"asset-management-api/assetpb"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type AreaService struct {
	DB *pgxpool.Pool
	assetpb.UnimplementedAREAServiceServer
}

func NewAreaService(db *pgxpool.Pool) *AreaService {
	return &AreaService{DB: db}
}

func (s *AreaService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterAREAServiceServer(grpcServer, s)
}

// ListArea: Mengambil daftar area menggunakan raw SQL
func (s *AreaService) ListArea(ctx context.Context, req *assetpb.ListAreaRequest) (*assetpb.ListAreaResponse, error) {
	log.Info().Msg("Fetching list of areas")
	rows, err := s.DB.Query(ctx, "SELECT area_id, area_name FROM areas")
	if err != nil {
		log.Error().Err(err).Msg("Error executing query")
		return &assetpb.ListAreaResponse{
			Data:    nil,
			Message: "Error fetching data",
			Code:    "500",
		}, nil
	}
	defer rows.Close()

	var areas []*assetpb.Area
	for rows.Next() {
		var area assetpb.Area
		err := rows.Scan(&area.AreaId, &area.AreaName)
		if err != nil {
			log.Error().Err(err).Msg("Error scanning row")
			return &assetpb.ListAreaResponse{
				Data:    nil,
				Message: "Error scanning row",
				Code:    "500",
			}, nil
		}
		areas = append(areas, &area)
	}

	if err = rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating over rows")
		return &assetpb.ListAreaResponse{
			Data:    nil,
			Message: "Error iterating over rows",
			Code:    "500",
		}, nil
	}

	log.Info().Msg("Successfully fetched list of areas")
	return &assetpb.ListAreaResponse{
		Data:    areas,
		Message: "Success",
		Code:    "200",
	}, nil
}

// CreateArea: Menambahkan area baru menggunakan raw SQL
func (s *AreaService) CreateArea(ctx context.Context, req *assetpb.CreateAreaRequest) (*assetpb.CreateAreaResponse, error) {
	query := "INSERT INTO areas (area_name) VALUES ($1)"
	_, err := s.DB.Exec(ctx, query, req.GetAreaName())

	if err != nil {
		log.Error().Err(err).Msg("Error creating area")
		return &assetpb.CreateAreaResponse{
			Message: "Error creating area",
			Code:    "500",
		}, nil
	}

	log.Info().Msg("Successfully created area")
	return &assetpb.CreateAreaResponse{
		Message: "Success",
		Code:    "200",
	}, nil
}
