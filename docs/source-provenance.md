# Source provenance

The rewrite was initialized from destination commit `3173b35` and imported from
the existing checkout without modifying it.

| Item          | Recorded value                                                                                                                                                   |
| ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Source path   | `/Users/maged/code/RSP-Web`                                                                                                                                      |
| Source commit | `8391ba51f4d56226c4ac28afd9748e83183f9016`                                                                                                                       |
| Source tree   | `13d4a47b2164cb3fc898ffe16383a748157fd2c6`                                                                                                                       |
| Dirty paths   | `client/src/pages/MockInterviews/MockInterview.page.tsx`; `client/src/pages/MockInterviews/MockInterviewTable/MockInterviewTable.tsx`; `docs/product-backlog.md` |

## SHA-256 at import

```text
952748e7c3dbb1924ed5a358244404372985ea3a04d0078d9a9b609f3fe4ba34  client/src/pages/MockInterviews/MockInterview.page.tsx
68eae3734870ae3dcd89d8d320e2f6500d2daf815269e62a525d903e51d41c03  client/src/pages/MockInterviews/MockInterviewTable/MockInterviewTable.tsx
35cd637ae913407ec2c870999d0b98dca42dac7a08c4723022addd55fd0ca523  docs/product-backlog.md
```

The two dirty TypeScript files were recorded by path and checksum above, then
translated into the framework-independent semantic fixtures. Their Mantine
source is intentionally not retained in the runtime repository. Branding and
favicon files were copied to `apps/web/public`. The legacy `.git`, C# source,
secrets, dependency/build directories, and Railway configuration were not
imported.

The complete valid-workflow baseline derived from this evidence is documented
in [`semantic-parity.md`](semantic-parity.md), with checked cases in
[`golden/legacy-workflows.json`](golden/legacy-workflows.json). Known legacy
defects are deliberately separated in [`known-defects.md`](known-defects.md).
