package safety

// Options captures global safety-related toggles from CLI flags.
type Options struct {
    DryRun bool
    Yes    bool
    Force  bool
}

