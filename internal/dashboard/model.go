package dashboard

type Summary struct {
	Builds          BuildCounts      `json:"builds"`
	Hosts           HostCounts       `json:"hosts"`
	Registries      RegistryCounts   `json:"registries"`
	Artifacts       ArtifactCounts   `json:"artifacts"`
	Projects        ProjectCounts    `json:"projects"`
	RecentTasks     []RecentTask     `json:"recentTasks"`
	RecentArtifacts []RecentArtifact `json:"recentArtifacts"`
}

type BuildCounts struct {
	Running int `json:"running"`
	Queued  int `json:"queued"`
	Failed  int `json:"failed"`
	Total   int `json:"total"`
}

type HostCounts struct {
	Online   int `json:"online"`
	Disabled int `json:"disabled"`
	Total    int `json:"total"`
}

type RegistryCounts struct {
	Available int `json:"available"`
	Disabled  int `json:"disabled"`
	Total     int `json:"total"`
}

type ArtifactCounts struct {
	Pushed int `json:"pushed"`
	Total  int `json:"total"`
}

type ProjectCounts struct {
	Active int `json:"active"`
	Total  int `json:"total"`
}

type RecentTask struct {
	ID           string  `json:"id"`
	ImageRef     string  `json:"imageRef"`
	Status       string  `json:"status"`
	Architecture string  `json:"architecture"`
	HostName     *string `json:"hostName"`
	CreatedAt    string  `json:"createdAt"`
}

type RecentArtifact struct {
	ID           string  `json:"id"`
	ImageRef     string  `json:"imageRef"`
	Digest       *string `json:"digest"`
	Architecture string  `json:"architecture"`
	ProjectName  string  `json:"projectName"`
	RegistryName string  `json:"registryName"`
	CreatedAt    string  `json:"createdAt"`
}
