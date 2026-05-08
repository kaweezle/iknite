package util

// cSpell: words pflag

import (
	"errors"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type failingValue struct {
	setErr error
	value  string
}

func (f *failingValue) String() string {
	return f.value
}

func (f *failingValue) Set(value string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.value = value
	return nil
}

func (f *failingValue) Type() string {
	return "string"
}

type failingSliceValue struct {
	replaceErr error
	values     []string
}

func (f *failingSliceValue) String() string {
	return ""
}

func (f *failingSliceValue) Set(value string) error {
	f.values = append(f.values, value)
	return nil
}

func (f *failingSliceValue) Type() string {
	return "stringSlice"
}

func (f *failingSliceValue) Append(value string) error {
	f.values = append(f.values, value)
	return nil
}

func (f *failingSliceValue) Replace(values []string) error {
	if f.replaceErr != nil {
		return f.replaceErr
	}
	f.values = append([]string(nil), values...)
	return nil
}

func (f *failingSliceValue) GetSlice() []string {
	return append([]string(nil), f.values...)
}

func TestToStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   any
		name    string
		wantErr string
		want    []string
	}{
		{name: "string slice", input: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "comma separated string", input: "a,b", want: []string{"a", "b"}},
		{name: "generic slice", input: []any{"a", 2, true}, want: []string{"a", "2", "true"}},
		{name: "invalid type", input: 42, wantErr: "expected slice value, got int"},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			got, err := toStringSlice(tc.input)
			if tc.wantErr != "" {
				req.ErrorContains(err, tc.wantErr)
				return
			}

			req.NoError(err)
			req.Equal(tc.want, got)
		})
	}
}

func TestBindFlagValue_ReturnsReplaceErrorForSliceValue(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("my_slice", []string{"a", "b"})

	flag := &pflag.Flag{Name: "my-slice", Value: &failingSliceValue{replaceErr: errors.New("replace failed")}}

	err := BindFlagValue(flag, v, "my_slice")
	req.ErrorContains(err, "while replacing viper array value for my-slice from viper key my_slice")
	req.ErrorContains(err, "replace failed")
}

func TestBindFlagValue_ReturnsConversionErrorForInvalidSliceInput(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("my_slice", 42)

	flag := &pflag.Flag{Name: "my-slice", Value: &failingSliceValue{}}

	err := BindFlagValue(flag, v, "my_slice")
	req.ErrorContains(err, "while getting viper array value for my_slice")
	req.ErrorContains(err, "expected slice value, got int")
}

func TestBindFlagValue_ReturnsSetErrorForScalarValue(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("my_flag", "from-viper")

	flag := &pflag.Flag{Name: "my-flag", Value: &failingValue{setErr: errors.New("set failed")}}

	err := BindFlagValue(flag, v, "my_flag")
	req.ErrorContains(err, "while setting viper value for my-flag from viper key my_flag")
	req.ErrorContains(err, "set failed")
}
