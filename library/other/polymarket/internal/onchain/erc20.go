// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// On-chain helpers for Polygon: RPC client construction, balance/allowance
// reads, ERC-20 approve broadcast, and ERC-1155 setApprovalForAll
// broadcast. All write paths are explicit (no implicit broadcast).
//
// Polymarket settlement contracts pinned here (PUBLIC, verifiable on
// Polygonscan):
//
//   USDC.e collateral:           0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174
//   CTF (Conditional Token Fwk): 0x4D97DCd97eC945f40cF65F87097ACe5EA0476045
//   CTF Exchange (Binary):       0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E
//   NegRisk CTF Exchange:        0xC5d563A36AE78145C45a50134d48A1215220f80a
//   NegRisk Adapter (Resolution):0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296

package onchain

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	PolygonChainID            = 137
	USDCe                     = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"
	CTFAddr                   = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"
	CTFExchangeAddr           = "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E"
	NegRiskCTFExchangeAddr    = "0xC5d563A36AE78145C45a50134d48A1215220f80a"
	NegRiskAdapterAddr        = "0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296"
)

// MaxUint256 is the canonical "infinite approval" amount used by every
// ERC-20 wrapper. Equivalent to 2^256 - 1.
var MaxUint256 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

// Dial wraps ethclient.DialContext with a sensible default timeout via the
// passed context. Caller owns the returned client and MUST call Close() in
// a defer to release the underlying websocket/http transport.
func Dial(ctx context.Context, rpcURL string) (*ethclient.Client, error) {
	if rpcURL == "" {
		return nil, errors.New("RPC URL is empty (set POLYGON_RPC_URL)")
	}
	return ethclient.DialContext(ctx, rpcURL)
}

// BalanceERC20 reads the token balance for `holder` from the standard
// balanceOf(address) method. Returns 0 + nil on missing-contract responses
// (RPC returns empty data), non-nil error on transport failures only.
func BalanceERC20(ctx context.Context, client *ethclient.Client, token, holder common.Address) (*big.Int, error) {
	data := append(
		crypto.Keccak256([]byte("balanceOf(address)"))[:4],
		padAddress(holder)...,
	)
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("balanceOf call: %w", err)
	}
	if len(out) == 0 {
		return big.NewInt(0), nil
	}
	return new(big.Int).SetBytes(out), nil
}

// AllowanceERC20 reads the current ERC-20 allowance granted by `owner` to
// `spender`.
func AllowanceERC20(ctx context.Context, client *ethclient.Client, token, owner, spender common.Address) (*big.Int, error) {
	data := append(
		crypto.Keccak256([]byte("allowance(address,address)"))[:4],
		padAddress(owner)...,
	)
	data = append(data, padAddress(spender)...)
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("allowance call: %w", err)
	}
	if len(out) == 0 {
		return big.NewInt(0), nil
	}
	return new(big.Int).SetBytes(out), nil
}

// ApproveERC20 broadcasts approve(spender, amount) on `token`. Returns the
// submitted tx hash; the caller can pass it to WaitMined to block until
// the receipt confirms.
func ApproveERC20(ctx context.Context, client *ethclient.Client, pk *ecdsa.PrivateKey, token, spender common.Address, amount *big.Int) (common.Hash, error) {
	auth, err := newTransactor(ctx, client, pk)
	if err != nil {
		return common.Hash{}, err
	}
	data := append(
		crypto.Keccak256([]byte("approve(address,uint256)"))[:4],
		padAddress(spender)...,
	)
	data = append(data, padUint256(amount)...)

	return sendTx(ctx, client, auth, token, big.NewInt(0), data, 80_000)
}

// RedeemCTF broadcasts redeemPositions(collateralToken, parentCollectionId,
// conditionId, indexSets) on the ConditionalTokens (CTF) contract. For
// Polymarket binary markets:
//
//   collateralToken    = USDC.e
//   parentCollectionId = 0x0 (top-level market, no parent)
//   conditionId        = market's CTF condition ID (bytes32)
//   indexSets          = [1, 2] (redeem both YES and NO; CTF auto-skips
//                        outcomes the caller holds zero balance of)
//
// For neg-risk markets the caller should target NegRiskAdapter instead of
// CTF — pass ctfAddr = NegRiskAdapterAddr.
//
// Returns the tx hash on broadcast success. If the wallet holds zero of
// every outcome token, the tx will mine but `payout` will be zero — no
// USDC transferred, gas spent regardless.
func RedeemCTF(ctx context.Context, client *ethclient.Client, pk *ecdsa.PrivateKey, ctfAddr, collateralToken common.Address, conditionId common.Hash, indexSets []*big.Int) (common.Hash, error) {
	auth, err := newTransactor(ctx, client, pk)
	if err != nil {
		return common.Hash{}, err
	}
	// redeemPositions(IERC20,bytes32,bytes32,uint256[]) selector = 0x2eb2c2d6
	// Per Solidity ABI: collateralToken=20bytes-left-padded-to-32, parentCollectionId=32, conditionId=32,
	// indexSets array: head (offset=0x80) → length → elements.
	data := []byte{0x2e, 0xb2, 0xc2, 0xd6}
	data = append(data, padAddress(collateralToken)...)
	// parentCollectionId = bytes32(0)
	data = append(data, make([]byte, 32)...)
	// conditionId
	data = append(data, conditionId.Bytes()...)
	// indexSets array offset (= 4 fixed-size 32-byte heads → offset = 4*32 = 128 = 0x80)
	data = append(data, padUint256(big.NewInt(0x80))...)
	// indexSets length
	data = append(data, padUint256(big.NewInt(int64(len(indexSets))))...)
	// indexSets elements
	for _, s := range indexSets {
		data = append(data, padUint256(s)...)
	}
	return sendTx(ctx, client, auth, ctfAddr, big.NewInt(0), data, 250_000)
}

// SetApprovalForAllERC1155 broadcasts setApprovalForAll(operator, approved)
// on `nft`. Used to authorize CTF Exchange to move the maker's outcome
// tokens during SELL settlement.
func SetApprovalForAllERC1155(ctx context.Context, client *ethclient.Client, pk *ecdsa.PrivateKey, nft, operator common.Address, approved bool) (common.Hash, error) {
	auth, err := newTransactor(ctx, client, pk)
	if err != nil {
		return common.Hash{}, err
	}
	data := append(
		crypto.Keccak256([]byte("setApprovalForAll(address,bool)"))[:4],
		padAddress(operator)...,
	)
	flag := big.NewInt(0)
	if approved {
		flag.SetInt64(1)
	}
	data = append(data, padUint256(flag)...)
	return sendTx(ctx, client, auth, nft, big.NewInt(0), data, 80_000)
}

// IsApprovedForAllERC1155 reads the current approval flag.
func IsApprovedForAllERC1155(ctx context.Context, client *ethclient.Client, nft, owner, operator common.Address) (bool, error) {
	data := append(
		crypto.Keccak256([]byte("isApprovedForAll(address,address)"))[:4],
		padAddress(owner)...,
	)
	data = append(data, padAddress(operator)...)
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &nft, Data: data}, nil)
	if err != nil {
		return false, fmt.Errorf("isApprovedForAll call: %w", err)
	}
	if len(out) < 32 {
		return false, nil
	}
	// uint256 0 = false, anything else = true
	return new(big.Int).SetBytes(out).Sign() != 0, nil
}

// WaitMinedByHash is the hash-based receipt waiter. We poll
// eth_getTransactionReceipt because go-ethereum's bind.WaitMined wants a
// *Transaction (which the caller may not have hand if they only know the
// hash from a prior tx). Returns once status is set or after `maxBlocks`
// elapse beyond the current head.
func WaitMinedByHash(ctx context.Context, client *ethclient.Client, hash common.Hash) (*types.Receipt, error) {
	queryTicker := defaultPollInterval
	timer := newTimer(queryTicker)
	defer timer.Stop()
	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			return receipt, nil
		}
		if err != nil && !errors.Is(err, ethereum.NotFound) {
			return nil, err
		}
		select {
		case <-timer.C:
			timer.Reset(queryTicker)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// PolygonscanLink builds the canonical block explorer URL for a tx hash.
func PolygonscanLink(hash common.Hash) string {
	return "https://polygonscan.com/tx/" + hash.Hex()
}

// newTransactor packages a bind.TransactOpts ready for EIP-1559 sends on
// Polygon mainnet. We pin chain ID 137 — Amoy testnet would need a
// different value and is out of scope for this build.
func newTransactor(ctx context.Context, client *ethclient.Client, pk *ecdsa.PrivateKey) (*bind.TransactOpts, error) {
	auth, err := bind.NewKeyedTransactorWithChainID(pk, big.NewInt(PolygonChainID))
	if err != nil {
		return nil, fmt.Errorf("new transactor: %w", err)
	}
	auth.Context = ctx
	auth.GasLimit = 0 // 0 = let bind estimate; overridden where we know cheap fixed gas
	return auth, nil
}

// sendTx assembles an EIP-1559 transaction (DynamicFeeTx), signs with the
// transactor's signer, broadcasts, and returns the tx hash. Gas estimation
// uses the live RPC; if estimation fails (often happens for contract
// methods the RPC mis-decodes) we fall back to the caller's gasLimit.
func sendTx(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, to common.Address, value *big.Int, data []byte, fallbackGas uint64) (common.Hash, error) {
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		return common.Hash{}, fmt.Errorf("get nonce: %w", err)
	}
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return common.Hash{}, fmt.Errorf("get latest header: %w", err)
	}
	// Polygon prefers EIP-1559 since the Bor upgrade. Use the base fee from
	// the head and bump tip by 2 gwei to land within ~1 block. Tip floor
	// keeps the tx priority above the Polygon mempool's ambient noise.
	baseFee := head.BaseFee
	if baseFee == nil {
		// Pre-1559 fallback (won't happen on Polygon mainnet, but keeps the
		// code robust for sandbox/test environments that strip the field).
		gp, err := client.SuggestGasPrice(ctx)
		if err != nil {
			return common.Hash{}, fmt.Errorf("suggest gas price: %w", err)
		}
		baseFee = gp
	}
	tip := big.NewInt(30_000_000_000) // 30 gwei tip — Polygon needs ≥30 gwei priority on mainnet
	maxFee := new(big.Int).Add(new(big.Int).Mul(baseFee, big.NewInt(2)), tip)

	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From: auth.From, To: &to, Value: value, Data: data,
	})
	if err != nil {
		gas = fallbackGas
	}
	if gas < 21_000 {
		gas = 21_000
	}

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(PolygonChainID),
		Nonce:     nonce,
		GasTipCap: tip,
		GasFeeCap: maxFee,
		Gas:       gas,
		To:        &to,
		Value:     value,
		Data:      data,
	})
	signed, err := auth.Signer(auth.From, tx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("sign tx: %w", err)
	}
	if err := client.SendTransaction(ctx, signed); err != nil {
		return common.Hash{}, fmt.Errorf("broadcast tx: %w", err)
	}
	return signed.Hash(), nil
}

// padAddress left-pads a 20-byte address to 32 bytes for ABI encoding.
func padAddress(a common.Address) []byte {
	padded := make([]byte, 32)
	copy(padded[12:], a.Bytes())
	return padded
}

// padUint256 left-pads a *big.Int to 32 bytes for ABI encoding.
func padUint256(n *big.Int) []byte {
	padded := make([]byte, 32)
	b := n.Bytes()
	copy(padded[32-len(b):], b)
	return padded
}
