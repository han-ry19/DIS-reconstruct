package domain

import (
	"fmt"
	"unicode/utf8"
)

// Limits are measured in UTF-8 bytes and apply before a request changes state.
const (
	MaxAddressBytes            = 1024
	MaxProfileLabelBytes       = 256
	MaxProfileFormatBytes      = 128
	MaxProfileDescriptionBytes = 4096
	MaxParameterBytes          = 8192
)

func validateParameterSizes(p Parameters) error {
	fields := []struct {
		name, value string
		limit       int
	}{
		{"field", p.Field, 64}, {"new_content", p.NewContent, MaxParameterBytes},
		{"authorization_owner", p.AuthorizationOwner, 128}, {"identity_id", p.IdentityID, 128},
		{"public_key", p.PublicKey, 128}, {"kind", p.Kind, 32}, {"data_id", p.DataID, 128},
		{"address", p.Address, MaxAddressBytes}, {"data_signature", p.DataSignature, 128},
		{"recipient", p.Recipient, 128}, {"status", string(p.Status), 32},
	}
	if p.Profile != nil {
		fields = append(fields, struct {
			name, value string
			limit       int
		}{"profile.label", p.Profile.Label, MaxProfileLabelBytes},
			struct {
				name, value string
				limit       int
			}{"profile.format", p.Profile.Format, MaxProfileFormatBytes},
			struct {
				name, value string
				limit       int
			}{"profile.description", p.Profile.Description, MaxProfileDescriptionBytes})
	}
	for _, f := range fields {
		if len(f.value) > f.limit || !utf8.ValidString(f.value) {
			return fmt.Errorf("%w: %s exceeds its UTF-8 byte limit or is invalid UTF-8", ErrInvalid, f.name)
		}
	}
	if len(p.Permissions) > 4 {
		return fmt.Errorf("%w: too many permissions", ErrInvalid)
	}
	for _, permission := range p.Permissions {
		if len(permission) > 16 || !utf8.ValidString(permission) {
			return ErrInvalid
		}
	}
	return nil
}
