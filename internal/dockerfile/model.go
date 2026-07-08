package dockerfile

type CopyRule struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type GenerateRequest struct {
	BaseImage   string            `json:"baseImage"`
	Environment map[string]string `json:"environment"`
	Workdir     string            `json:"workdir"`
	Packages    []string          `json:"packages"`
	Copy        []CopyRule        `json:"copy"`
	Expose      []int             `json:"expose"`
	CMD         []string          `json:"cmd"`
	Entrypoint  []string          `json:"entrypoint"`
	Args        map[string]string `json:"args"`
	Labels      map[string]string `json:"labels"`
}

type ValidateRequest struct {
	Dockerfile string `json:"dockerfile"`
}

type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`
}
