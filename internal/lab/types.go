package lab

type Composition struct {
	SchemaVersion int          `json:"schema_version"`
	ID            string       `json:"id"`
	Stage         int          `json:"stage"`
	CoreContract  CoreContract `json:"core_contract"`
	Axes          []string     `json:"axes"`
	Subjects      []SubjectRef `json:"subjects"`
	Profiles      []string     `json:"profiles"`
	Scenarios     []string     `json:"scenarios"`
}

type CoreContract struct {
	Repository    string `json:"repository"`
	Commit        string `json:"commit"`
	PolicyVersion string `json:"policy_version"`
}
type SubjectRef struct {
	Name              string `json:"name"`
	SubjectID         string `json:"subject_id"`
	Version           string `json:"version"`
	ReleaseManifest   string `json:"release_manifest"`
	ReleaseDigest     string `json:"release_digest"`
	CertificateDigest string `json:"certificate_digest"`
}

type Release struct {
	SchemaVersion int         `json:"schema_version"`
	SubjectID     string      `json:"subject_id"`
	Version       string      `json:"version"`
	Status        string      `json:"status"`
	Artifact      ArtifactRef `json:"artifact"`
	Launch        Launch      `json:"launch"`
	Interfaces    []string    `json:"interfaces"`
	Certificate   string      `json:"certificate"`
}
type ArtifactRef struct {
	Path      string `json:"path"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
}
type Launch struct {
	Driver string   `json:"driver"`
	Args   []string `json:"args"`
	Port   int      `json:"port"`
}
type SubjectCertificate struct {
	SchemaVersion  int    `json:"schema_version"`
	SubjectID      string `json:"subject_id"`
	Version        string `json:"version"`
	Status         string `json:"status"`
	ArtifactDigest string `json:"artifact_digest"`
	IssuedAt       string `json:"issued_at"`
}

type Scenario struct {
	SchemaVersion int            `json:"schema_version"`
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Axes          []string       `json:"axes"`
	Oracle        string         `json:"oracle"`
	Phases        ScenarioPhases `json:"phases"`
}
type ScenarioPhases struct {
	Setup   []Action `json:"setup"`
	Execute []Action `json:"execute"`
	Verify  []Action `json:"verify"`
	Cleanup []Action `json:"cleanup"`
}
type Action struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Service string            `json:"service,omitempty"`
	Method  string            `json:"method,omitempty"`
	Path    string            `json:"path,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
	Expect  *Expectation      `json:"expect,omitempty"`
	Capture map[string]string `json:"capture,omitempty"`
	Left    string            `json:"left,omitempty"`
	Right   string            `json:"right,omitempty"`
	Op      string            `json:"op,omitempty"`
}
type Expectation struct {
	Status int             `json:"status"`
	JSON   []JSONAssertion `json:"json,omitempty"`
}
type JSONAssertion struct {
	Path  string `json:"path"`
	Op    string `json:"op"`
	Value any    `json:"value,omitempty"`
}

type Environment struct {
	SchemaVersion   int      `json:"schema_version"`
	Profile         string   `json:"profile"`
	Required        []string `json:"required"`
	Isolation       []string `json:"isolation"`
	Cleanup         []string `json:"cleanup"`
	CloudDependency string   `json:"cloud_dependency"`
}

type ActionResult struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Status     int    `json:"status,omitempty"`
	Assertions int    `json:"assertions"`
	Verdict    string `json:"verdict"`
	Error      string `json:"error,omitempty"`
}
type ScenarioReport struct {
	SchemaVersion     int            `json:"schema_version"`
	ScenarioID        string         `json:"scenario_id"`
	Profile           string         `json:"profile"`
	CompositionDigest string         `json:"composition_digest"`
	ScenarioDigest    string         `json:"scenario_digest"`
	Axes              []string       `json:"axes"`
	Actions           []ActionResult `json:"actions"`
	Verdict           string         `json:"verdict"`
}
type RunSummary struct {
	Profile           string   `json:"profile"`
	CompositionDigest string   `json:"composition_digest"`
	ScenarioReports   []string `json:"scenario_reports"`
	EvidenceSetDigest string   `json:"evidence_set_digest"`
	Verdict           string   `json:"verdict"`
	CleanupReceipt    string   `json:"cleanup_receipt"`
}
type CleanupReceipt struct {
	SchemaVersion        int    `json:"schema_version"`
	Profile              string `json:"profile"`
	CompositionID        string `json:"composition_id"`
	Processes            int    `json:"remaining_processes"`
	Containers           int    `json:"remaining_containers"`
	Networks             int    `json:"remaining_networks"`
	Images               int    `json:"remaining_images"`
	CredentialsPersisted bool   `json:"credentials_persisted"`
	Verdict              string `json:"verdict"`
}

var RequiredAxes = []string{"communication", "identity", "data", "messaging", "deployment", "observability", "security-boundary", "failure-propagation", "recovery", "compatibility"}
