package tmservice

import (
	"context"
	"crypto/sha256"

	gogogrpc "github.com/gogo/protobuf/grpc"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/line/lbm-sdk/client"
	"github.com/line/lbm-sdk/client/rpc"
	codectypes "github.com/line/lbm-sdk/codec/types"
	cryptotypes "github.com/line/lbm-sdk/crypto/types"
	qtypes "github.com/line/lbm-sdk/types/query"
	"github.com/line/lbm-sdk/version"
)

// This is the struct that we will implement all the handlers on.
type queryServer struct {
	clientCtx         client.Context
	interfaceRegistry codectypes.InterfaceRegistry
}

var (
	_ ServiceServer                      = queryServer{}
	_ codectypes.UnpackInterfacesMessage = &GetLatestValidatorSetResponse{}
)

// NewQueryServer creates a new tendermint query server.
func NewQueryServer(clientCtx client.Context, interfaceRegistry codectypes.InterfaceRegistry) ServiceServer {
	return queryServer{
		clientCtx:         clientCtx,
		interfaceRegistry: interfaceRegistry,
	}
}

// GetSyncing implements ServiceServer.GetSyncing
func (s queryServer) GetSyncing(ctx context.Context, _ *GetSyncingRequest) (*GetSyncingResponse, error) {
	status, err := getNodeStatus(ctx, s.clientCtx)
	if err != nil {
		return nil, err
	}
	return &GetSyncingResponse{
		Syncing: status.SyncInfo.CatchingUp,
	}, nil
}

// GetLatestBlock implements ServiceServer.GetLatestBlock
func (s queryServer) GetLatestBlock(ctx context.Context, _ *GetLatestBlockRequest) (*GetLatestBlockResponse, error) {
	status, err := getBlock(ctx, s.clientCtx, nil)
	if err != nil {
		return nil, err
	}

	protoBlockID := status.BlockID.ToProto()
	protoBlock, err := status.Block.ToProto()
	if err != nil {
		return nil, err
	}

	return &GetLatestBlockResponse{
		BlockId: &protoBlockID,
		Block:   protoBlock,
	}, nil
}

// GetBlockByHeight implements ServiceServer.GetBlockByHeight
func (s queryServer) GetBlockByHeight(ctx context.Context, req *GetBlockByHeightRequest) (*GetBlockByHeightResponse, error) {
	chainHeight, err := rpc.GetChainHeight(s.clientCtx)
	if err != nil {
		return nil, err
	}

	if req.Height > chainHeight {
		return nil, status.Error(codes.InvalidArgument, "requested block height is bigger then the chain length")
	}

	protoBlockID, protoBlock, err := GetProtoBlock(ctx, s.clientCtx, &req.Height)
	if err != nil {
		return nil, err
	}
	return &GetBlockByHeightResponse{
		BlockId: &protoBlockID,
		Block:   protoBlock,
	}, nil
}

// GetBlockByHash implements ServiceServer.GetBlockByHash
func (s queryServer) GetBlockByHash(_ context.Context, req *GetBlockByHashRequest) (*GetBlockByHashResponse, error) {
	if n := len(req.Hash); n != sha256.Size {
		if n == 0 {
			return nil, status.Error(codes.InvalidArgument, "block hash cannot be empty")
		}
		return nil, status.Error(codes.InvalidArgument, "the length of block hash must be 32")
	}

	res, err := getBlockByHash(s.clientCtx, req.Hash)
	if err != nil {
		return nil, err
	}
	protoBlockID := res.BlockID.ToProto()
	protoBlock, err := res.Block.ToProto()
	if err != nil {
		return nil, err
	}
	return &GetBlockByHashResponse{
		BlockId: &protoBlockID,
		Block:   protoBlock,
	}, nil
}

// GetBlockResultsByHeight implements ServiceServer.GetBlockResultsByHeight
func (s queryServer) GetBlockResultsByHeight(_ context.Context, req *GetBlockResultsByHeightRequest) (*GetBlockResultsByHeightResponse, error) {
	res, err := getBlockResultsByHeight(s.clientCtx, &req.Height)
	if err != nil {
		return nil, err
	}
	return &GetBlockResultsByHeightResponse{
		Height:     res.Height,
		TxsResults: res.TxsResults,
		ResBeginBlock: &abci.ResponseBeginBlock{
			Events: res.BeginBlockEvents,
		},
		ResEndBlock: &abci.ResponseEndBlock{
			ValidatorUpdates:      res.ValidatorUpdates,
			ConsensusParamUpdates: res.ConsensusParamUpdates,
			Events:                res.EndBlockEvents,
		},
	}, nil
}

// GetLatestValidatorSet implements ServiceServer.GetLatestValidatorSet
func (s queryServer) GetLatestValidatorSet(ctx context.Context, req *GetLatestValidatorSetRequest) (*GetLatestValidatorSetResponse, error) {
	page, limit, err := qtypes.ParsePagination(req.Pagination)
	if err != nil {
		return nil, err
	}
	return validatorsOutput(ctx, s.clientCtx, nil, page, limit)
}

func (m *GetLatestValidatorSetResponse) UnpackInterfaces(unpacker codectypes.AnyUnpacker) error {
	var pubKey cryptotypes.PubKey
	for _, val := range m.Validators {
		err := unpacker.UnpackAny(val.PubKey, &pubKey)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetValidatorSetByHeight implements ServiceServer.GetValidatorSetByHeight
func (s queryServer) GetValidatorSetByHeight(ctx context.Context, req *GetValidatorSetByHeightRequest) (*GetValidatorSetByHeightResponse, error) {
	page, limit, err := qtypes.ParsePagination(req.Pagination)
	if err != nil {
		return nil, err
	}

	chainHeight, err := rpc.GetChainHeight(s.clientCtx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to parse chain height")
	}
	if req.Height > chainHeight {
		return nil, status.Error(codes.InvalidArgument, "requested block height is bigger then the chain length")
	}
	r, err := validatorsOutput(ctx, s.clientCtx, &req.Height, page, limit)
	if err != nil {
		return nil, err
	}
	return &GetValidatorSetByHeightResponse{
		BlockHeight: r.BlockHeight,
		Validators:  r.Validators,
		Pagination:  r.Pagination,
	}, nil
}

func validatorsOutput(ctx context.Context, cctx client.Context, height *int64, page, limit int) (*GetLatestValidatorSetResponse, error) {
	vs, err := rpc.GetValidators(ctx, cctx, height, &page, &limit)
	if err != nil {
		return nil, err
	}
	resp := GetLatestValidatorSetResponse{
		BlockHeight: vs.BlockHeight,
		Validators:  make([]*Validator, len(vs.Validators)),
		Pagination: &qtypes.PageResponse{
			Total: vs.Total,
		},
	}
	for i, v := range vs.Validators {
		anyPub, err := codectypes.NewAnyWithValue(v.PubKey)
		if err != nil {
			return nil, err
		}
		resp.Validators[i] = &Validator{
			Address:          v.Address.String(),
			ProposerPriority: v.ProposerPriority,
			PubKey:           anyPub,
			VotingPower:      v.VotingPower,
		}
	}
	return &resp, nil
}

// GetNodeInfo implements ServiceServer.GetNodeInfo
func (s queryServer) GetNodeInfo(ctx context.Context, req *GetNodeInfoRequest) (*GetNodeInfoResponse, error) {
	status, err := getNodeStatus(ctx, s.clientCtx)
	if err != nil {
		return nil, err
	}

	protoNodeInfo := status.NodeInfo.ToProto()
	nodeInfo := version.NewInfo()

	deps := make([]*Module, len(nodeInfo.BuildDeps))

	for i, dep := range nodeInfo.BuildDeps {
		deps[i] = &Module{
			Path:    dep.Path,
			Sum:     dep.Sum,
			Version: dep.Version,
		}
	}

	resp := GetNodeInfoResponse{
		DefaultNodeInfo: protoNodeInfo,
		ApplicationVersion: &VersionInfo{
			AppName:       nodeInfo.AppName,
			Name:          nodeInfo.Name,
			GitCommit:     nodeInfo.GitCommit,
			GoVersion:     nodeInfo.GoVersion,
			Version:       nodeInfo.Version,
			BuildTags:     nodeInfo.BuildTags,
			BuildDeps:     deps,
			LbmSdkVersion: nodeInfo.LbmSdkVersion,
		},
	}
	return &resp, nil
}

// RegisterTendermintService registers the tendermint queries on the gRPC router.
func RegisterTendermintService(
	qrt gogogrpc.Server,
	clientCtx client.Context,
	interfaceRegistry codectypes.InterfaceRegistry,
) {
	RegisterServiceServer(
		qrt,
		NewQueryServer(clientCtx, interfaceRegistry),
	)
}

// RegisterGRPCGatewayRoutes mounts the tendermint service's GRPC-gateway routes on the
// given Mux.
func RegisterGRPCGatewayRoutes(clientConn gogogrpc.ClientConn, mux *runtime.ServeMux) {
	RegisterServiceHandlerClient(context.Background(), mux, NewServiceClient(clientConn))
}
