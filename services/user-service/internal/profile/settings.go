package profile

func applySettingsUpdate(
	current Settings,
	input SettingsUpdateInput,
) (Settings, bool, error) {
	updated := current
	changed := false

	if input.Theme != nil {
		if err := validateTheme(*input.Theme); err != nil {
			return Settings{}, false, err
		}
		if *input.Theme != updated.Theme {
			updated.Theme = *input.Theme
			changed = true
		}
	}
	if input.Autoplay != nil && *input.Autoplay != updated.Autoplay {
		updated.Autoplay = *input.Autoplay
		changed = true
	}
	if input.ExplicitContent != nil && *input.ExplicitContent != updated.ExplicitContent {
		updated.ExplicitContent = *input.ExplicitContent
		changed = true
	}
	if input.AudioQuality != nil {
		if err := validateAudioQuality(*input.AudioQuality); err != nil {
			return Settings{}, false, err
		}
		if *input.AudioQuality != updated.AudioQuality {
			updated.AudioQuality = *input.AudioQuality
			changed = true
		}
	}
	if input.NormalizeVolume != nil && *input.NormalizeVolume != updated.NormalizeVolume {
		updated.NormalizeVolume = *input.NormalizeVolume
		changed = true
	}
	if input.ShowListeningActivity != nil &&
		*input.ShowListeningActivity != updated.ShowListeningActivity {
		updated.ShowListeningActivity = *input.ShowListeningActivity
		changed = true
	}

	return updated, changed, nil
}

func validateTheme(theme InterfaceTheme) error {
	switch theme {
	case ThemeLight, ThemeDark, ThemeSystem:
		return nil
	default:
		return invalid("theme", "must be light, dark, or system")
	}
}

func validateAudioQuality(quality AudioQuality) error {
	switch quality {
	case AudioQualityLow, AudioQualityMedium, AudioQualityHigh, AudioQualityLossless:
		return nil
	default:
		return invalid("audio_quality", "must be low, medium, high, or lossless")
	}
}
