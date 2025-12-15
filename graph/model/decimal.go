package model

import (
	"fmt"
	"io"
	"strconv"

	"github.com/shopspring/decimal"
)

type Decimal decimal.Decimal

func (d *Decimal) UnmarshalGQL(v interface{}) error {
	var strVal string
	
	switch val := v.(type) {
	case string:
		strVal = val
	case int, int8, int16, int32, int64:
        strVal = fmt.Sprint(val)
    default:
		return fmt.Errorf("decimal must be given as an int or a string, received %T", v)	
	}

	dec, err := decimal.NewFromString(strVal)
	if err != nil {
		return fmt.Errorf("incorrect decimal format: %w", err)
	}

	*d = Decimal(dec)
	return nil
}

func (d Decimal) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(decimal.Decimal(d).String()))
}