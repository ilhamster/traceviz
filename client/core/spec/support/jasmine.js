export default {
  reporters: [
    {
      name: "jasmine-spec-reporter#SpecReporter",
      options: {
        displayStacktrace: "all"
      }
    }
  ],
  spec_dir: "src",
  spec_files: ["**/*_test.ts"]
};
