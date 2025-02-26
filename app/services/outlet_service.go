package services

import (
	"asset-management-api/assetpb"
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type OutletService struct {
    MasterService
    assetpb.UnimplementedOUTLETServiceServer
    DB *pgxpool.Pool
}

func NewOutletService(db *pgxpool.Pool) *OutletService {
    return &OutletService{
        MasterService: MasterService{DB: db},
        DB:            db,
    }
}

func (s *OutletService) Register(server interface{}) {
    grpcServer := server.(grpc.ServiceRegistrar)
    assetpb.RegisterOUTLETServiceServer(grpcServer, s)
}

func (s *OutletService) ListOutlet(ctx context.Context, req *assetpb.ListOutletRequest) (*assetpb.ListOutletResponse, error) {
    log.Info().Msg("List outlet")
    var outlets []*assetpb.Outlet
    var rows pgx.Rows
    var err error

    if req.AreaId != 0 {
        log.Info().Msgf("List outlet with area ID: %d", req.AreaId)
        query := "SELECT o.outlet_id, o.outlet_name FROM outlets o JOIN area_outlets ao ON o.outlet_id = ao.outlet_id WHERE ao.area_id = $1"
        rows, err = s.DB.Query(ctx, query, req.AreaId)
    } else {
        query := "SELECT outlet_id, outlet_name FROM outlets"
        rows, err = s.DB.Query(ctx, query)
    }

    if err != nil {
        log.Error().Err(err).Msgf("Error fetching data for area_id: %d", req.AreaId)
        return &assetpb.ListOutletResponse{
            Data:    nil,
            Message: "Error fetching data: " + err.Error(),
            Code:    "500",
        }, nil
    }

    defer rows.Close()

    for rows.Next() {
        var outlet assetpb.Outlet
        if err := rows.Scan(&outlet.OutletId, &outlet.OutletName); err != nil {
            log.Error().Err(err).Msg("Error scanning row")
            continue
        }
        outlets = append(outlets, &outlet)
    }

    return &assetpb.ListOutletResponse{
        Data:    outlets,
        Message: "Success",
        Code:    "200",
    }, nil
}

func (s *OutletService) CreateOutlet(ctx context.Context, req *assetpb.CreateOutletRequest) (*assetpb.CreateOutletResponse, error) {
    query := "INSERT INTO outlets (outlet_name) VALUES ($1) RETURNING outlet_id"
    var outletId int64
    err := s.DB.QueryRow(ctx, query, req.GetOutletName()).Scan(&outletId)
    if err != nil {
        log.Error().Err(err).Msg("Error creating data outlet")
        return &assetpb.CreateOutletResponse{
            Message: "Error creating data outlet",
            Code:    "500",
        }, nil
    }

    areaQuery := "INSERT INTO area_outlets (area_id, outlet_id) VALUES ($1, $2)"
    _, err = s.DB.Exec(ctx, areaQuery, req.GetAreaId(), outletId)
    if err != nil {
        log.Error().Err(err).Msg("Error creating data area outlet")
        return &assetpb.CreateOutletResponse{
            Message: "Error creating data area outlet",
            Code:    "500",
        }, nil
    }

    return &assetpb.CreateOutletResponse{
        Message: "Success",
        Code:    "200",
    }, nil
}