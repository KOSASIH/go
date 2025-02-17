package stellarcore

const (
	// PreflightStatusError represents the status value returned by stellar-core when an error occurred from
	// processing a preflight request
	PreflightStatusError = "ERROR"

	// PreflightStatusOk represents the status value returned by stellar-core when a preflight request
	// succeeded
	PreflightStatusOk = "OK"
)

// PreflightResponse is the response from Stellar Core for the preflight endpoint
type PreflightResponse struct {
	Status          string `json:"status"`
	Detail          string `json:"detail"`
	Result          string `json:"result"`
	Footprint       string `json:"footprint"`
	CPUInstructions uint64 `json:"cpu_insns"`
	MemoryBytes     uint64 `json:"mem_bytes"`
	Ledger          int64  `json:"ledger"`
}
