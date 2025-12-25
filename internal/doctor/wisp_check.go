package doctor

// Legacy wisp directory checks removed.
// Wisps are now just a flag on regular beads issues (Wisp: true).
// Hook files are stored in .beads/ alongside other beads data.
//
// These checks were for the old .beads-wisp/ directory infrastructure:
// - WispExistsCheck: checked if .beads-wisp/ exists
// - WispGitCheck: checked if .beads-wisp/ is a git repo
// - WispOrphansCheck: checked for old wisps
// - WispSizeCheck: checked size of .beads-wisp/
// - WispStaleCheck: checked for inactive wisps
//
// All removed as of the wisp simplification (gt-5klh, bd-bkul).
