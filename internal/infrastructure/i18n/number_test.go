package i18n

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInt_GroupsDigitsPerLocale(t *testing.T) {
	// given
	en := WithLocale(context.Background(), Normalize("en"))
	pl := WithLocale(context.Background(), Normalize("pl"))

	// when
	gotEN, gotPL := Int(en, 1234567), Int(pl, 1234567)

	// then
	assert.Equal(t, "1,234,567", gotEN)
	assert.Equal(t, "1 234 567", gotPL)
}

func TestDecimal_UsesLocaleSeparatorAndFixedDigits(t *testing.T) {
	// given
	en := WithLocale(context.Background(), Normalize("en"))
	pl := WithLocale(context.Background(), Normalize("pl"))

	// when
	gotEN, gotPL := Decimal(en, 1234.5, 1), Decimal(pl, 2, 1)

	// then
	assert.Equal(t, "1,234.5", gotEN)
	assert.Equal(t, "2,0", gotPL)
}
