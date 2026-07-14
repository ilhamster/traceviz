export default {
  reporters: [
    {
      name: "jasmine-spec-reporter#SpecReporter",
      options: {
        displayStacktrace: "all",
      },
    },
  ],
  spec_dir: ".",
  spec_files: ["components/trace/trace_minimap_dom_test.tsx"],
};
