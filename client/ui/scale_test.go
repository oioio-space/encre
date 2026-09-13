package ui_test

import (
	"testing"

	"github.com/oioio-space/encre/client/ui"
)

func TestWindowScaleIsTheLargestWholeMultipleThatFits(t *testing.T) {
	tests := []struct {
		name               string
		logicalW, logicalH int
		monitorW, monitorH int
		want               int
	}{
		{
			name:     "portrait barely fits once on a 1080p desktop",
			logicalW: 390, logicalH: 844, monitorW: 1920, monitorH: 1080,
			want: 1,
		},
		{
			name:     "the same portrait doubles on a 4K desktop",
			logicalW: 390, logicalH: 844, monitorW: 3840, monitorH: 2160,
			want: 2,
		},
		{
			name:     "landscape triples on a 4K desktop",
			logicalW: 640, logicalH: 360, monitorW: 3840, monitorH: 2160,
			want: 5,
		},
		{
			name:     "a monitor smaller than the screen still gives 1",
			logicalW: 390, logicalH: 844, monitorW: 320, monitorH: 480,
			want: 1,
		},
		{
			name: "an unknown monitor gives 1 rather than 0",
			// Ebitengine reports 0x0 before the window exists; a zero scale
			// would ask for a window with no pixels in it.
			logicalW: 390, logicalH: 844, monitorW: 0, monitorH: 0,
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ui.WindowScale(tt.logicalW, tt.logicalH, tt.monitorW, tt.monitorH)
			if got != tt.want {
				t.Errorf("WindowScale(%d, %d, %d, %d) = %d, want %d",
					tt.logicalW, tt.logicalH, tt.monitorW, tt.monitorH, got, tt.want)
			}
		})
	}
}

func TestWindowScaleLeavesRoomForTheWindowChrome(t *testing.T) {
	// A window exactly as tall as the monitor loses its title bar behind the
	// panel, and on some desktops cannot be moved back.
	const monitorH = 900
	scale := ui.WindowScale(390, 844, 1920, monitorH)

	if 844*scale >= monitorH {
		t.Errorf("WindowScale gave %d, making a %d px window on a %d px monitor",
			scale, 844*scale, monitorH)
	}
}

func TestPhysicalPxScalesByWindowRatioThenDeviceScale(t *testing.T) {
	tests := []struct {
		name                        string
		logical, screenLen, outside int
		deviceScale                 float64
		want                        float64
	}{
		{
			name:    "screen drawn 1:1 into the window, scale 3",
			logical: 52, screenLen: 390, outside: 390, deviceScale: 3,
			want: 156,
		},
		{
			name:    "logical screen squeezed into a narrower window",
			logical: 52, screenLen: 390, outside: 360, deviceScale: 2,
			want: 96,
		},
		{
			name:    "desktop window, no device scaling",
			logical: 52, screenLen: 390, outside: 390, deviceScale: 1,
			want: 52,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ui.PhysicalPx(tt.logical, tt.screenLen, tt.outside, tt.deviceScale)
			if got != tt.want {
				t.Errorf("PhysicalPx(%d, %d, %d, %v) = %v, want %v",
					tt.logical, tt.screenLen, tt.outside, tt.deviceScale, got, tt.want)
			}
		})
	}
}

func TestPhysicalPxIsZeroWhenTheScreenHasNoWidth(t *testing.T) {
	// Ebitengine calls Layout before the window exists; a division by zero here
	// would poison the measurement the prototype exists to report.
	if got := ui.PhysicalPx(52, 0, 390, 3); got != 0 {
		t.Errorf("PhysicalPx with screenLen 0 = %v, want 0", got)
	}
}
