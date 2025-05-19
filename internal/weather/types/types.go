package types

type Weather struct {
	Temp        float64 `json:"temp"`
	Humidity    int     `json:"humidity"`
	Description string  `json:"description"`
}
