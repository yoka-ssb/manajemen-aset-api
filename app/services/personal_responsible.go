package services

import (
	"asset-management-api/assetpb"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type PersonalResponsibleService struct {
	MasterService
	assetpb.UnimplementedPERSONALRESPONSIBLEServiceServer
	DB *pgxpool.Pool
}

func NewPersonalResponsibleService(db *pgxpool.Pool) *PersonalResponsibleService {
	return &PersonalResponsibleService{
		MasterService: MasterService{},
		DB:            db,
	}
}

func (s *PersonalResponsibleService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterPERSONALRESPONSIBLEServiceServer(grpcServer, s)
}
func (s *PersonalResponsibleService) ListPersonalResponsible(ctx context.Context, req *assetpb.ListPersonalResponsibleRequest) (*assetpb.ListPersonalResponsibleResponse, error) {
	log.Info().Msg("Getting all personal responsibles")

	query := "SELECT personal_responsible_id, name FROM personal_responsibles"
	rows, err := s.DB.Query(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching data")
		return &assetpb.ListPersonalResponsibleResponse{
			Data:    nil,
			Message: "Error fetching data",
			Code:    "500",
		}, nil
	}
	defer rows.Close()

	var personalResponsibles []*assetpb.PersonalResponsible
	for rows.Next() {
		var personalResponsible assetpb.PersonalResponsible
		if err := rows.Scan(&personalResponsible.PersonalId, &personalResponsible.PersonalName); err != nil {
			log.Error().Err(err).Msg("Error scanning row")
			continue
		}
		personalResponsibles = append(personalResponsibles, &personalResponsible)
	}

	return &assetpb.ListPersonalResponsibleResponse{
		Data:    personalResponsibles,
		Message: "Success",
		Code:    "200",
	}, nil
}
