package errors

import (
	"reflect"
	"testing"
)

func TestHedgeError_Error(t *testing.T) {
	type fields struct {
		errorType ErrorType
		message   string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "errorType and message is filled out", fields: fields{errorType: ErrorTypeConflict, message: "error message"}, want: "error message",
		},
		{
			name: "message is empty", fields: fields{errorType: ErrorTypeConflict, message: ""}, want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := CommonHedgeError{
				errorType: tt.fields.errorType,
				message:   tt.fields.message,
			}
			if got := h.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewHedgeError(t *testing.T) {
	type args struct {
		errorType ErrorType
		message   string
	}
	tests := []struct {
		name string
		args args
		want CommonHedgeError
	}{
		{
			name: "error type and message are filled out",
			args: args{errorType: ErrorTypeConflict, message: "error message"},
			want: CommonHedgeError{errorType: ErrorTypeConflict, message: "error message"},
		},
		{
			name: "message is empty",
			args: args{errorType: ErrorTypeConflict, message: ""},
			want: CommonHedgeError{errorType: ErrorTypeConflict, message: ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewCommonHedgeError(tt.args.errorType, tt.args.message); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewCommonHedgeError() = %v, want %v", got, tt.want)
			}
		})
	}
}
