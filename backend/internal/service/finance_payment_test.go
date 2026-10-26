//go:build unit

package service

// CUSTOM(fork): 通道成本快照与用户承担的手续费独立。
import (
	"context"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestFinanceChannelFeeValidation(t *testing.T) {
	for _, v := range []string{"-0.1", "0.2001", "NaN", "0.03201"} {
		_, err := ValidateChannelFeeRate(v)
		require.Error(t, err)
	}
	for _, v := range []string{"", "0", "0.0320", "0.2"} {
		_, err := ValidateChannelFeeRate(v)
		require.NoError(t, err)
	}
}
func TestFinancePaymentFeeSnapshotAndRefund(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	u, err := client.User.Create().SetEmail("finance-snapshot@example.com").SetPasswordHash("hash").Save(ctx)
	require.NoError(t, err)
	inst, err := client.PaymentProviderInstance.Create().SetProviderKey(payment.TypeEasyPay).SetConfig("{}").SetChannelFeeRate("0.0320").Save(ctx)
	require.NoError(t, err)
	svc := &PaymentService{entClient: client}
	o, err := svc.createOrderInTx(ctx, CreateOrderRequest{UserID: u.ID, OrderType: "balance", PaymentType: "wxpay"}, &User{ID: u.ID, Email: u.Email}, nil, &PaymentConfig{MaxPendingOrders: 5, OrderTimeoutMin: 30}, 10, 10, 0, 10, 0, &payment.InstanceSelection{InstanceID: strconv.FormatInt(inst.ID, 10), ProviderKey: payment.TypeEasyPay})
	require.NoError(t, err)
	require.NotNil(t, o.ChannelFeeRate)
	require.Equal(t, "0.0320", *o.ChannelFeeRate)
	_, err = client.PaymentProviderInstance.UpdateOneID(inst.ID).SetChannelFeeRate("0.0100").Save(ctx)
	require.NoError(t, err)
	saved, err := client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.Equal(t, "0.0320", *saved.ChannelFeeRate)
	require.Equal(t, 10.0, saved.PayAmount)
	_, err = svc.markRefundOkTx(ctx, client, &RefundPlan{Order: o, OrderID: o.ID, RefundAmount: 3, BalanceToDeduct: 0})
	require.NoError(t, err)
	saved, err = client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.NotNil(t, saved.RefundBalanceDeducted)
	require.Equal(t, "0.00000000", *saved.RefundBalanceDeducted)
}

func TestFinancePaymentProviderCRUD(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentConfigService{entClient: client, encryptionKey: []byte("0123456789abcdef0123456789abcdef")}
	inst, err := svc.CreateProviderInstance(ctx, CreateProviderInstanceRequest{ProviderKey: "easypay", Name: "财务测试", Config: map[string]string{}, ChannelFeeRate: "0.0320"})
	require.NoError(t, err)
	require.Equal(t, "0.0320", inst.ChannelFeeRate)
	inst, err = svc.UpdateProviderInstance(ctx, inst.ID, UpdateProviderInstanceRequest{})
	require.NoError(t, err)
	require.Equal(t, "0.0320", inst.ChannelFeeRate)
	rate := "0.01"
	inst, err = svc.UpdateProviderInstance(ctx, inst.ID, UpdateProviderInstanceRequest{ChannelFeeRate: &rate})
	require.NoError(t, err)
	require.Equal(t, "0.0100", inst.ChannelFeeRate)
	list, err := svc.ListProviderInstancesWithConfig(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "0.0100", list[0].ChannelFeeRate)
	rate = "0.21"
	_, err = svc.UpdateProviderInstance(ctx, inst.ID, UpdateProviderInstanceRequest{ChannelFeeRate: &rate})
	require.Error(t, err)
}
