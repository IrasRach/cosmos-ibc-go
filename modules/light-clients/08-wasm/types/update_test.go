package types_test

import (
	"encoding/json"
	"fmt"

	wasmvm "github.com/CosmWasm/wasmvm"
	wasmvmtypes "github.com/CosmWasm/wasmvm/types"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"

	wasmtesting "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/testing"
	"github.com/cosmos/ibc-go/modules/light-clients/08-wasm/types"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	host "github.com/cosmos/ibc-go/v8/modules/core/24-host"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"
	tmtypes "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"
)

func (suite *TypesTestSuite) TestUpdateState() {
	mockHeight := clienttypes.NewHeight(1, 1)

	var (
		clientMsg      exported.ClientMessage
		clientStore    storetypes.KVStore
		expClientState *types.ClientState
	)

	testCases := []struct {
		name       string
		malleate   func()
		expPanic   error
		expHeights []exported.Height
	}{
		{
			"success: no update",
			func() {
				suite.mockVM.RegisterSudoCallback(types.UpdateStateMsg{}, func(_ wasmvm.Checksum, env wasmvmtypes.Env, sudoMsg []byte, store wasmvm.KVStore, _ wasmvm.GoAPI, _ wasmvm.Querier, _ wasmvm.GasMeter, _ uint64, _ wasmvmtypes.UFraction) (*wasmvmtypes.Response, uint64, error) {
					var msg types.SudoMsg
					err := json.Unmarshal(sudoMsg, &msg)
					suite.Require().NoError(err)

					suite.Require().NotNil(msg.UpdateState)
					suite.Require().NotNil(msg.UpdateState.ClientMessage)
					suite.Require().Equal(msg.UpdateState.ClientMessage.Data, wasmtesting.MockClientStateBz)
					suite.Require().Nil(msg.VerifyMembership)
					suite.Require().Nil(msg.VerifyNonMembership)
					suite.Require().Nil(msg.UpdateStateOnMisbehaviour)
					suite.Require().Nil(msg.VerifyUpgradeAndUpdateState)

					suite.Require().Equal(env.Contract.Address, defaultWasmClientID)

					updateStateResp := types.UpdateStateResult{
						Heights: []clienttypes.Height{},
					}

					resp, err := json.Marshal(updateStateResp)
					if err != nil {
						return nil, 0, err
					}

					return &wasmvmtypes.Response{
						Data: resp,
					}, wasmtesting.DefaultGasUsed, nil
				})
			},
			nil,
			[]exported.Height{},
		},
		{
			"success: update client",
			func() {
				data := []byte("new-client-state-data")
				expClientState.Data = data
				clientMsg = &types.ClientMessage{
					Data: data,
				}

				suite.mockVM.RegisterSudoCallback(types.UpdateStateMsg{}, func(_ wasmvm.Checksum, env wasmvmtypes.Env, sudoMsg []byte, store wasmvm.KVStore, _ wasmvm.GoAPI, _ wasmvm.Querier, _ wasmvm.GasMeter, _ uint64, _ wasmvmtypes.UFraction) (*wasmvmtypes.Response, uint64, error) {
					var msg types.SudoMsg
					err := json.Unmarshal(sudoMsg, &msg)
					suite.Require().NoError(err)

					bz := store.Get(host.ClientStateKey())
					suite.Require().NotEmpty(bz)
					clientState := clienttypes.MustUnmarshalClientState(suite.chainA.App.AppCodec(), bz).(*types.ClientState)
					clientState.Data = msg.UpdateState.ClientMessage.Data
					store.Set(host.ClientStateKey(), clienttypes.MustMarshalClientState(suite.chainA.App.AppCodec(), clientState))

					updateStateResp := types.UpdateStateResult{
						Heights: []clienttypes.Height{mockHeight},
					}

					resp, err := json.Marshal(updateStateResp)
					if err != nil {
						return nil, 0, err
					}

					return &wasmvmtypes.Response{
						Data: resp,
					}, wasmtesting.DefaultGasUsed, nil
				})
			},
			nil,
			[]exported.Height{mockHeight},
		},
		{
			"failure: clientStore prefix does not include clientID",
			func() {
				clientStore = suite.chainA.App.GetIBCKeeper().ClientKeeper.ClientStore(suite.ctx, ibctesting.InvalidID)
			},
			errorsmod.Wrap(types.ErrWasmContractCallFailed, errorsmod.Wrap(errorsmod.Wrapf(types.ErrRetrieveClientID, "prefix does not contain a valid clientID: %s", errorsmod.Wrapf(host.ErrInvalidID, "invalid client identifier %s", ibctesting.InvalidID)), "failed to retrieve clientID for wasm contract call").Error()),
			nil,
		},
		{
			"failure: invalid ClientMessage type",
			func() {
				// SudoCallback left nil because clientMsg is checked by 08-wasm before callbackFn is called.
				clientMsg = &tmtypes.Misbehaviour{}
			},
			fmt.Errorf("expected type %T, got %T", (*types.ClientMessage)(nil), (*tmtypes.Misbehaviour)(nil)),
			nil,
		},
		{
			"failure: callbackFn returns error",
			func() {
				suite.mockVM.RegisterSudoCallback(types.UpdateStateMsg{}, func(_ wasmvm.Checksum, _ wasmvmtypes.Env, _ []byte, _ wasmvm.KVStore, _ wasmvm.GoAPI, _ wasmvm.Querier, _ wasmvm.GasMeter, _ uint64, _ wasmvmtypes.UFraction) (*wasmvmtypes.Response, uint64, error) {
					return nil, 0, wasmtesting.ErrMockContract
				})
			},
			errorsmod.Wrap(types.ErrWasmContractCallFailed, wasmtesting.ErrMockContract.Error()),
			nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupWasmWithMockVM() // reset

			clientMsg = &types.ClientMessage{
				Data: wasmtesting.MockClientStateBz,
			}

			endpoint := wasmtesting.NewWasmEndpoint(suite.chainA)
			err := endpoint.CreateClient()
			suite.Require().NoError(err)
			clientStore = suite.chainA.App.GetIBCKeeper().ClientKeeper.ClientStore(suite.chainA.GetContext(), endpoint.ClientID)

			tc.malleate()

			//clientState := endpoint.GetClientState()
			expClientState = endpoint.GetClientState().(*types.ClientState)

			// bz := clientStore.Get(host.ClientStateKey())

			// css, _ := suite.chainA.App.GetIBCKeeper().ClientKeeper.GetClientState(suite.chainA.GetContext(), endpoint.ClientID)
			// suite.Require().NotNil(css)

			// var cs types.ClientState
			// err = suite.chainA.Codec.UnmarshalInterface(bz, &cs)

			var heights []exported.Height
			updateState := func() {
				heights = expClientState.UpdateState(suite.chainA.GetContext(), suite.chainA.Codec, clientStore, clientMsg)
			}

			if tc.expPanic == nil {
				updateState()
				suite.Require().Equal(tc.expHeights, heights)

				if expClientState != nil {
					clientState := endpoint.GetClientState()
					suite.Require().NoError(err)
					suite.Require().Equal(expClientState, clientState)
				}
			} else {
				suite.Require().PanicsWithError(tc.expPanic.Error(), updateState)
			}
		})
	}
}

func (suite *TypesTestSuite) TestUpdateStateOnMisbehaviour() {
	var clientMsg exported.ClientMessage

	testCases := []struct {
		name               string
		malleate           func()
		panicErr           error
		updatedClientState []byte
	}{
		{
			"success: no update",
			func() {
				suite.mockVM.RegisterSudoCallback(types.UpdateStateOnMisbehaviourMsg{}, func(_ wasmvm.Checksum, _ wasmvmtypes.Env, sudoMsg []byte, store wasmvm.KVStore, _ wasmvm.GoAPI, _ wasmvm.Querier, _ wasmvm.GasMeter, _ uint64, _ wasmvmtypes.UFraction) (*wasmvmtypes.Response, uint64, error) {
					var msg types.SudoMsg

					err := json.Unmarshal(sudoMsg, &msg)
					suite.Require().NoError(err)

					suite.Require().NotNil(msg.UpdateStateOnMisbehaviour)
					suite.Require().NotNil(msg.UpdateStateOnMisbehaviour.ClientMessage)
					suite.Require().Nil(msg.UpdateState)
					suite.Require().Nil(msg.UpdateState)
					suite.Require().Nil(msg.VerifyMembership)
					suite.Require().Nil(msg.VerifyNonMembership)
					suite.Require().Nil(msg.VerifyUpgradeAndUpdateState)

					resp, err := json.Marshal(types.EmptyResult{})
					if err != nil {
						return nil, 0, err
					}

					return &wasmvmtypes.Response{
						Data: resp,
					}, wasmtesting.DefaultGasUsed, nil
				})
			},
			nil,
			nil,
		},
		{
			"success: client state updated on valid misbehaviour",
			func() {
				suite.mockVM.RegisterSudoCallback(types.UpdateStateOnMisbehaviourMsg{}, func(_ wasmvm.Checksum, _ wasmvmtypes.Env, sudoMsg []byte, store wasmvm.KVStore, _ wasmvm.GoAPI, _ wasmvm.Querier, _ wasmvm.GasMeter, _ uint64, _ wasmvmtypes.UFraction) (*wasmvmtypes.Response, uint64, error) {
					var msg types.SudoMsg
					err := json.Unmarshal(sudoMsg, &msg)
					suite.Require().NoError(err)

					// set new client state in store
					store.Set(host.ClientStateKey(), msg.UpdateStateOnMisbehaviour.ClientMessage.Data)
					resp, err := json.Marshal(types.EmptyResult{})
					if err != nil {
						return nil, 0, err
					}

					return &wasmvmtypes.Response{Data: resp}, wasmtesting.DefaultGasUsed, nil
				})
			},
			nil,
			wasmtesting.MockClientStateBz,
		},
		{
			"failure: invalid client message",
			func() {
				clientMsg = &tmtypes.Header{}
				// we will not register the callback here because this test case does not reach the VM
			},
			fmt.Errorf("expected type %T, got %T", (*types.ClientMessage)(nil), (*tmtypes.Header)(nil)),
			nil,
		},
		{
			"failure: err return from contract vm",
			func() {
				suite.mockVM.RegisterSudoCallback(types.UpdateStateOnMisbehaviourMsg{}, func(_ wasmvm.Checksum, _ wasmvmtypes.Env, _ []byte, store wasmvm.KVStore, _ wasmvm.GoAPI, _ wasmvm.Querier, _ wasmvm.GasMeter, _ uint64, _ wasmvmtypes.UFraction) (*wasmvmtypes.Response, uint64, error) {
					return nil, 0, wasmtesting.ErrMockContract
				})
			},
			errorsmod.Wrap(types.ErrWasmContractCallFailed, wasmtesting.ErrMockContract.Error()),
			nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// reset suite to create fresh application state
			suite.SetupWasmWithMockVM()

			endpoint := wasmtesting.NewWasmEndpoint(suite.chainA)
			err := endpoint.CreateClient()
			suite.Require().NoError(err)

			store := suite.chainA.App.GetIBCKeeper().ClientKeeper.ClientStore(suite.chainA.GetContext(), endpoint.ClientID)
			clientMsg = &types.ClientMessage{
				Data: wasmtesting.MockClientStateBz,
			}
			clientState := endpoint.GetClientState()

			tc.malleate()

			if tc.panicErr == nil {
				clientState.UpdateStateOnMisbehaviour(suite.chainA.GetContext(), suite.chainA.App.AppCodec(), store, clientMsg)
				if tc.updatedClientState != nil {
					suite.Require().Equal(tc.updatedClientState, store.Get(host.ClientStateKey()))
				}
			} else {
				suite.Require().PanicsWithError(tc.panicErr.Error(), func() {
					clientState.UpdateStateOnMisbehaviour(suite.chainA.GetContext(), suite.chainA.App.AppCodec(), store, clientMsg)
				})
			}
		})
	}
}
