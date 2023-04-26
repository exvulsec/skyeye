package model

type MetaDockLabelRequest struct {
	Chain     string   `json:"chain"`
	Addresses []string `json:"addresses"`
}

type MetaDockLabelResponse struct {
	Address string `json:"address"`
	Label   string `json:"label"`
}

type MetaDockLabelsResponse []MetaDockLabelResponse
