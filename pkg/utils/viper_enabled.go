package utils

import "github.com/spf13/viper"

type ViperProvider interface {
	Viper() *viper.Viper
}

type ViperHolder interface {
	SetViper(*viper.Viper)
}

type ViperEnabled struct {
	v *viper.Viper
}

var _ ViperProvider = (*ViperEnabled)(nil)

var _ ViperHolder = (*ViperEnabled)(nil)

func NewViperEnabled(v *viper.Viper) *ViperEnabled {
	return &ViperEnabled{v: v}
}

func (ve *ViperEnabled) Viper() *viper.Viper {
	if ve.v == nil {
		ve.v = viper.New()
	}
	return ve.v
}

func (ve *ViperEnabled) SetViper(v *viper.Viper) {
	ve.v = v
}
