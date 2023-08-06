package outputs

import (
	"fmt"
	"strconv"
	"strings"
)

type Mode struct {
	Width              int64   `json:"width"`
	Height             int64   `json:"height"`
	Refresh            float64 `json:"refresh"`
	PictureAspectRatio string  `json:"picture_aspect_ratio"`
}

func (m *Mode) String() string {
	if m.Refresh != 0 {
		return fmt.Sprintf("%vx%v@%v", m.Width, m.Height, m.Refresh)
	} else {
		return fmt.Sprintf("%vx%v", m.Width, m.Height)
	}
}

func NewMode(s string) (*Mode, error) {

	// split an optional freqency
	items := strings.SplitN(s, "@", 2)
	xyStr := ""
	refreshStr := ""
	if len(items) == 1 {
		xyStr = items[0]
	} else if len(items) == 2 {
		xyStr = items[0]
		refreshStr = items[1]
	} else {
		return nil, fmt.Errorf("invalid mode: %v", s)
	}

	var refresh float64 = 0
	if refreshStr != "" {
		v, err := strconv.ParseFloat(refreshStr, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to parse float: %w", err)
		}
		refresh = v
	}

	// parse x and y
	xyItems := strings.SplitN(xyStr, "x", 2)
	if len(items) != 2 {
		return nil, fmt.Errorf("invalid XxY mode")
	}

	x, err := strconv.ParseInt(xyItems[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse x as int: %w", err)
	}
	y, err := strconv.ParseInt(xyItems[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse y as int: %w", err)
	}

	return &Mode{
		Width:              x,
		Height:             y,
		Refresh:            refresh,
		PictureAspectRatio: "",
	}, nil
}
