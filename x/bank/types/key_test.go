package types_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/line/lbm-sdk/types"
	"github.com/line/lbm-sdk/types/address"
	"github.com/line/lbm-sdk/x/bank/types"
)

func cloneAppend(bz []byte, tail []byte) (res []byte) {
	res = make([]byte, len(bz)+len(tail))
	copy(res, bz)
	copy(res[len(bz):], tail)
	return
}

func TestAddressFromBalancesStore(t *testing.T) {
	addr, err := sdk.AccAddressFromBech32("link19tzp7e489drh9qfs9m84k2qe5a5yyknzen48tz")
	require.NoError(t, err)
	addrLen := len(addr)
	require.Equal(t, 20, addrLen)
	key := cloneAppend(address.MustLengthPrefix(addr), []byte("stake"))

	tests := []struct {
		name        string
		key         []byte
		wantErr     bool
		expectedKey sdk.AccAddress
	}{
		{"valid", key, false, addr},
		{"#9111", []byte("\xff000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"), false, nil},
		{"empty", []byte(""), true, nil},
		{"invalid", []byte("3AA"), true, nil},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			addr, err := types.AddressFromBalancesStore(tc.key)
			if tc.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.Is(types.ErrInvalidKey, err))
			} else {
				assert.NoError(t, err)
			}
			if len(tc.expectedKey) > 0 {
				assert.Equal(t, tc.expectedKey, addr)
			}
		})
	}
}
