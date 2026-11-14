package source

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestValidateReferenceAcceptsSupportedSourceKinds(t *testing.T) {
	tests := []Reference{
		{Kind: KindLocalDevice, Metadata: map[string]string{"original_name": "track.flac"}},
		{Kind: KindServerStorage, Metadata: map[string]string{"collection": "demo"}},
		{
			Kind: KindExternalProvider, Provider: ProviderJamendo,
			ExternalID: "12345", SourceURL: "https://www.jamendo.com/track/12345",
			License: License{
				Name: "CC BY 3.0", URL: "https://creativecommons.org/licenses/by/3.0/",
				Attribution: "Example Artist", AllowsStreaming: true,
				AllowsRedistribution: true,
			},
			Metadata: map[string]string{"artist": "Example Artist"},
		},
	}
	for _, reference := range tests {
		if err := ValidateReference(reference); err != nil {
			t.Fatalf("ValidateReference(%s) error = %v", reference.Kind, err)
		}
	}
}

func TestValidateReferenceRejectsUnsafeOrInconsistentData(t *testing.T) {
	tests := []struct {
		name      string
		reference Reference
		want      error
	}{
		{
			name:      "unsupported kind",
			reference: Reference{Kind: "remote"},
			want:      ErrInvalid,
		},
		{
			name:      "provider fields on local source",
			reference: Reference{Kind: KindLocalDevice, Provider: ProviderJamendo},
			want:      ErrInvalid,
		},
		{
			name: "unsupported provider",
			reference: Reference{
				Kind: KindExternalProvider, Provider: "spotify", ExternalID: "track",
			},
			want: ErrUnsupportedProvider,
		},
		{
			name: "credentialed source URL",
			reference: Reference{
				Kind: KindExternalProvider, Provider: ProviderJamendo, ExternalID: "track",
				SourceURL: "https://user:secret@example.com/track",
				License:   License{Name: "CC BY", URL: "https://example.com/license"},
			},
			want: ErrInvalid,
		},
		{
			name: "oversized metadata",
			reference: Reference{
				Kind:     KindServerStorage,
				Metadata: map[string]string{"value": strings.Repeat("x", MaximumMetadataValueSize+1)},
			},
			want: ErrInvalid,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateReference(test.reference); !errors.Is(err, test.want) {
				t.Fatalf("ValidateReference() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestValidateAudioStream(t *testing.T) {
	valid := AudioStream{
		Body:     io.NopCloser(strings.NewReader("audio")),
		Filename: "track.flac", MIMEType: "audio/flac", SizeBytes: 5,
	}
	if err := ValidateAudioStream(valid); err != nil {
		t.Fatalf("ValidateAudioStream() error = %v", err)
	}
	invalid := valid
	invalid.MIMEType = "text/html"
	if err := ValidateAudioStream(invalid); !errors.Is(err, ErrInvalidStream) {
		t.Fatalf("ValidateAudioStream() error = %v, want %v", err, ErrInvalidStream)
	}
}
