# Changelog

## [0.1.3](https://github.com/TerraConstructs/grid/compare/v0.1.2...v0.1.3) (2025-12-04)


### Miscellaneous

* remove go mod replace ([#19](https://github.com/TerraConstructs/grid/issues/19)) ([0bbeaad](https://github.com/TerraConstructs/grid/commit/0bbeaad3d9432a09ace9b067fe2579dec8487173))

## [0.1.2](https://github.com/TerraConstructs/grid/compare/v0.1.1...v0.1.2) (2025-12-04)


### Bug Fixes

* **ci:** Fix tar command in GoReleaser before hook ([e489d3f](https://github.com/TerraConstructs/grid/commit/e489d3fea6f349478f74760dbb1018d96ad3e86e))

## [0.1.1](https://github.com/TerraConstructs/grid/compare/grid-v0.1.0...grid-v0.1.1) (2025-12-03)


### Features

* Add --link, --force to gridctl state get ([16a446c](https://github.com/TerraConstructs/grid/commit/16a446cbf56e5929a6f08210b1d061b885c653f6))
* Add CI/CD Workflows ([9cdb3d0](https://github.com/TerraConstructs/grid/commit/9cdb3d0b1aded7862406d1a339914a01a8f7f66e))
* Add CLI UX improvements ([4858e77](https://github.com/TerraConstructs/grid/commit/4858e77c4e85f151a50d5917cb293046273ceef8))
* Add count fields to Go SDK StateSummary ([ebae544](https://github.com/TerraConstructs/grid/commit/ebae54475d16fe8654a6b8fc5bd52cdb6af8b13e))
* Add dashboard PoC ([5d837dc](https://github.com/TerraConstructs/grid/commit/5d837dc6d6b96c89dc83ab6713398c37c5be5cd4))
* Add Edge Status updates ([d532976](https://github.com/TerraConstructs/grid/commit/d532976ad4f0b47ae15be35a9f9942522d4f402b))
* Add flexible eager loading methods and optimize GetStateInfo ([209f325](https://github.com/TerraConstructs/grid/commit/209f325ff852437fd5daf8217bdafb5bf008dfe2))
* Add JSON Schema support for State outputs ([d401c76](https://github.com/TerraConstructs/grid/commit/d401c768953a9f50d15c5381955abaf26688546a))
* Add JSON Schema support for State outputs ([f438e7d](https://github.com/TerraConstructs/grid/commit/f438e7d6a5b80ae53516cdbaecb1d2ab4090e744))
* Add no-auth e2e ([a72a50b](https://github.com/TerraConstructs/grid/commit/a72a50b56b2615ff604fa8a06f9e0354729c102c))
* Add output validation Job ([9d09556](https://github.com/TerraConstructs/grid/commit/9d09556d70584fa5854e3235b2f2eb7f72bb76c1))
* Add relationship count fields to eliminate frontend N+1 pattern ([13bb454](https://github.com/TerraConstructs/grid/commit/13bb454a6680e94d8150747220849e5dad46991c))
* add SQLite database support to gridapi ([d0086f3](https://github.com/TerraConstructs/grid/commit/d0086f34a60013d09d8b1b86c9c4906b981bf547))
* add SQLite database support to gridapi ([3eea58b](https://github.com/TerraConstructs/grid/commit/3eea58bed49e20ead3e1656358cb5bc7e935a87c))
* Add state dimensions ([f317f37](https://github.com/TerraConstructs/grid/commit/f317f374dc19ac4150f2ccbebf64446fb640565f))
* **auth:** Add AuthN/AuthZ and RBAC ([4b2d554](https://github.com/TerraConstructs/grid/commit/4b2d5544935774437a2b2ad5fc54954928d2fef8))
* **auth:** Add dependency auth tests and gridctl auth discovery ([9a4b70b](https://github.com/TerraConstructs/grid/commit/9a4b70b481a574b419a87a63badad235d34cb2ef))
* AuthN/AuthZ implementation with Mode 1 & Mode 2 support ([db274cb](https://github.com/TerraConstructs/grid/commit/db274cbcb336783d76e555a56bb754881ebddb42))
* Basic AuthN / AuthZ with internal IdP ([b54f577](https://github.com/TerraConstructs/grid/commit/b54f577e45e8f30034027d8c2b14f7121f3da4f5))
* Basic Repo set up and state mgmt ([74982ad](https://github.com/TerraConstructs/grid/commit/74982ad6090084c65b662c30743380612e35e3c0))
* Enable count fields and include_status parameter in handlers ([d85f63e](https://github.com/TerraConstructs/grid/commit/d85f63ee12d617b8d9ac3c5d982be85d9a73c943))
* first draft of 008-cicd-workflows implementation ([5f838ac](https://github.com/TerraConstructs/grid/commit/5f838ac439755aec064b63fdaccf0d49f2688145))
* First draft of dependency management ([f7671f8](https://github.com/TerraConstructs/grid/commit/f7671f8988d66a2681e0a718262a83e2cb33cc2a))
* **gridapi:** Adopt Viper ([96031f4](https://github.com/TerraConstructs/grid/commit/96031f4800f47aea8e31f0614593bf54d4c39448))
* **gridctl:** add -l shorthand for --label and --init flag ([1d3958c](https://github.com/TerraConstructs/grid/commit/1d3958c20a338d53d9956c9adab922f9c117d069))
* **gridctl:** improve gridctl UX ([bd3f957](https://github.com/TerraConstructs/grid/commit/bd3f957a97ea792b3a0c98b16e541e722c8fa040))
* **gridctl:** rename state schema commands and add shorthand flags ([7d64239](https://github.com/TerraConstructs/grid/commit/7d64239260fae4b6f7c8b46c7be2df29b6afd93c))
* Implement auto schema inference ([233ea59](https://github.com/TerraConstructs/grid/commit/233ea5963cc52132d0a3bd0e1fc74b5c44eeee8e))
* Implement Phase 3 Webapp UI for output schema display (grid-149f, grid-25e5, grid-7f81, grid-fb4e) ([44d3a68](https://github.com/TerraConstructs/grid/commit/44d3a68f2bafdcf21c027f65b6495979be2354c4))
* Initial AuthN/AuthZ draft ([2c7f87a](https://github.com/TerraConstructs/grid/commit/2c7f87a136a645aa8a4f1126d4cef9122725841f))
* Initial webapp auth tasks ([880b632](https://github.com/TerraConstructs/grid/commit/880b6322bde4edbb6c2454845571102abfb52fde))
* Initialize 008-cicd-workflows plan ([a79c06d](https://github.com/TerraConstructs/grid/commit/a79c06d580d6a5546733957effeedb050bdd1c9f))
* Integrate React Flow for interactive graph visualization ([1da75f9](https://github.com/TerraConstructs/grid/commit/1da75f90ac9bc860638295376c2eeb9157c1833a))
* Integrate ReactFlow into GraphView component ([374081c](https://github.com/TerraConstructs/grid/commit/374081c023abbfaae27bc08e2d3f048de652b0ca))
* labels implementation ([7c41ed8](https://github.com/TerraConstructs/grid/commit/7c41ed81a9211cdf3bc962074e32d6f34f341465))
* Optimize JS SDK and webapp to eliminate N+1 pattern ([4ef3632](https://github.com/TerraConstructs/grid/commit/4ef36326c1abacbb780ad79e3324d43d5c2d62d8))
* Optimize RPC handlers to eliminate network-level N+1 queries ([a548f0e](https://github.com/TerraConstructs/grid/commit/a548f0edc353d4482e5b0c5824aa14532b6c6dd1))
* Playwright e2e tests for web authentication ([#8](https://github.com/TerraConstructs/grid/issues/8)) ([9d274e0](https://github.com/TerraConstructs/grid/commit/9d274e082578d3867c9d5467d8db836f9c5ed370))
* Replace Dashboard mock with @tcons/grid ([195c610](https://github.com/TerraConstructs/grid/commit/195c610771e0850a10febcec9ff210cd2a737822))
* Updated AuthN/AuthZ after gap analysis ([dbb1e48](https://github.com/TerraConstructs/grid/commit/dbb1e4889be7c0146129fcb4c95a0cabcea1f519))
* WebApp User Login Flow ([039dae9](https://github.com/TerraConstructs/grid/commit/039dae9642b75e556b57ed22a2ee86db95439581))
* **webapp:** Add label filter component ([171b4f9](https://github.com/TerraConstructs/grid/commit/171b4f9b631b27b8f54a00c6dd3973feb56ef00c))


### Bug Fixes

* Add SVG arrowhead markers to show edge direction ([ac5ccba](https://github.com/TerraConstructs/grid/commit/ac5ccbacecff1cbaa006a2af121b73f99a04b07c))
* Address all Go linting issues (errcheck, unused, staticcheck) ([229f64e](https://github.com/TerraConstructs/grid/commit/229f64e55fcc08e68dbe36e8d81c2684df11deca))
* Address Go linting issues in gridctl module ([1eb4708](https://github.com/TerraConstructs/grid/commit/1eb47089313600d8aac4d223a81feda5e21cf602))
* Address Go linting issues in tests module ([88a47b9](https://github.com/TerraConstructs/grid/commit/88a47b96bd5dac931e98add206ef641a3d159c3c))
* **auth:** split dependency actions and add SIGHUP cache refresh ([eae68f8](https://github.com/TerraConstructs/grid/commit/eae68f84fda90876c4dc3bd14abc83b0b738fc6d))
* **auth:** split dependency list and list-all actions ([2c4104c](https://github.com/TerraConstructs/grid/commit/2c4104c86b00b53dc32ac35c7d529fe5c20b7255))
* **ci:** add missing packages section to release-please config ([b100509](https://github.com/TerraConstructs/grid/commit/b10050918347b6eb60d602cff2faf07a63873244))
* **ci:** add webapp/package.json to release-please extra-files ([139c3b8](https://github.com/TerraConstructs/grid/commit/139c3b89d20553bf5138ff54d655fd71f9b1fe39))
* **ci:** make frontend and go-lint checks non-blocking ([b22e730](https://github.com/TerraConstructs/grid/commit/b22e730afa35fabc22f5ca7b897627a6ce115d01))
* **ci:** restore RELEASE_PLEASE_TOKEN for workflow triggers ([98f0b7d](https://github.com/TerraConstructs/grid/commit/98f0b7d5d7550cd833063c75876b4c735710a595))
* **ci:** use default GITHUB_TOKEN for release-please ([61eb8d3](https://github.com/TerraConstructs/grid/commit/61eb8d3445b7f1ee058e32f11c94ebb8e39b9839))
* configure SQLite connection pool for in-memory databases ([12a531c](https://github.com/TerraConstructs/grid/commit/12a531c7d3f68b503a0e9efec0bc5fd44f155f2f))
* convert dialect.Name to string for comparisons ([bed770f](https://github.com/TerraConstructs/grid/commit/bed770fa40a9aef11381e496e514ff0da1ee1b0f))
* Correct return value assignments in unit tests ([3bbca46](https://github.com/TerraConstructs/grid/commit/3bbca464cfc30ad142ca6665d491bfac4eeb9800))
* **e2e:** adopt data-testid for reliable test selectors ([4425443](https://github.com/TerraConstructs/grid/commit/4425443616566039fccedd9161ece9c04bf094c0))
* ensure markerEdge is passed through ([c896fb2](https://github.com/TerraConstructs/grid/commit/c896fb2586802f259fb271cf31d4452b96114fd2))
* Ensure React Flow container has explicit dimensions ([13218e6](https://github.com/TerraConstructs/grid/commit/13218e6eb82f84ebad797d27ceeaa4ff3ead20aa))
* failing output schema integration tests ([a1575a4](https://github.com/TerraConstructs/grid/commit/a1575a45279688d122e4bed9baf6a95e752f8e79))
* GraphView canvas ([fd5b292](https://github.com/TerraConstructs/grid/commit/fd5b29297233367988e6e16c9f8fbc6b547096b5))
* **gridapi:** Fix OIDC user roles and state listing for scoped roles ([6756b17](https://github.com/TerraConstructs/grid/commit/6756b175394b612e0b5c8c32f0452861345da574))
* **gridapi:** Fix OIDC user roles and state listing for scoped roles ([e87787c](https://github.com/TerraConstructs/grid/commit/e87787ce42868903d34574667fb322607b4eb5ee))
* **gridapi:** go linting issues ([3d5670a](https://github.com/TerraConstructs/grid/commit/3d5670a35cc35e3e505276d54b356b028d929555))
* improve JSON schema validation and error messaging (grid-e903, grid-a966, grid-522d) ([981698b](https://github.com/TerraConstructs/grid/commit/981698b72a7fff146ae970a1b4a7d9f00bcc1d3e))
* Improve React Flow graph layout and edge distribution ([70e77d5](https://github.com/TerraConstructs/grid/commit/70e77d572952cd68bc3935b3b70abd6975b1a1d0))
* more fixes ([9cc12f5](https://github.com/TerraConstructs/grid/commit/9cc12f59e660f2f10f59a31a6ddae60dca172501))
* Move edge status legend to bottom-left to avoid overlap with minimap ([0f597d3](https://github.com/TerraConstructs/grid/commit/0f597d34a0c311290a354493eebe7aef49ecd647))
* Move legend to top-right to avoid overlapping Controls ([82d7bf5](https://github.com/TerraConstructs/grid/commit/82d7bf565801ec4f662f13de0e7974a2ebc5a54f))
* pnpm version and go linting ([6a4ef31](https://github.com/TerraConstructs/grid/commit/6a4ef3191a2c686aa0762e9b2841daa2aa5017d9))
* Preserve backward compatibility for include_status default ([b649a39](https://github.com/TerraConstructs/grid/commit/b649a3995161f808e1d3718ce2530b39678260f2))
* release please config ([848f4fa](https://github.com/TerraConstructs/grid/commit/848f4fa2a0cbe47f720fec48092dd342d6a8f37b))
* remove package-lock.json ([35354d2](https://github.com/TerraConstructs/grid/commit/35354d2b11b49f241594f4801ff5eaaeeb0ed905))
* Remove unused count methods and re-add TODOs for buf generate ([c9bad18](https://github.com/TerraConstructs/grid/commit/c9bad18d4ac96822ae420b62f40440ca4b7358ce))
* remove unused dialect imports in migrations helper ([d8d0300](https://github.com/TerraConstructs/grid/commit/d8d0300b763991b12981f6919ce62d5a98ac67a9))
* resolve multiple PR check failures ([e7ce65e](https://github.com/TerraConstructs/grid/commit/e7ce65ef57202b90a210094d380294571b7babb7))
* Restructure GraphView layout for proper height calculation ([f255976](https://github.com/TerraConstructs/grid/commit/f255976cad39622dd45f7929105ae4f4b5dfc14c))
* Return all states in no-auth mode and when role lookup fails ([522042a](https://github.com/TerraConstructs/grid/commit/522042a024d28761ed1b4c20f7028000518dc557))
* StateSummary transport and usage ([2a42442](https://github.com/TerraConstructs/grid/commit/2a42442901a34f523b44eb4d04ed47ffd40c81f8))
* use DATABASE_URL env var for SQLite integration tests ([b37a4d7](https://github.com/TerraConstructs/grid/commit/b37a4d78813fe16f33aa4d0ec029008a5d506b69))
* webapp auth flow ([f8f2c5f](https://github.com/TerraConstructs/grid/commit/f8f2c5ff16a8e166acdfc1a8e433a47b22d2ede2))


### Miscellaneous

* Add caveats on github actions ([3e40eef](https://github.com/TerraConstructs/grid/commit/3e40eef2e1ea2ef0aca2865e15762f9decf62355))
* Add make test-integration ([5286b2c](https://github.com/TerraConstructs/grid/commit/5286b2c1f290fa03b809708238d141ef5b353d64))
* Add SDK package-lock.json ([e1067f2](https://github.com/TerraConstructs/grid/commit/e1067f2c2964e574e80d1ce125c02e9e53d90cf2))
* analysis fixes ([53ae1cc](https://github.com/TerraConstructs/grid/commit/53ae1cc9bbfb001567628d47e88a649b89a591b2))
* clean up ([d887640](https://github.com/TerraConstructs/grid/commit/d887640b4bca93c3e1c0806612441a2e062e84a8))
* Clean up and finalize ([88c8588](https://github.com/TerraConstructs/grid/commit/88c858828ed0f68036f10a19bba40d03d79c3561))
* cleanup ([a4db725](https://github.com/TerraConstructs/grid/commit/a4db725cddd16b09d1bf1d9a3042f89d61bd9759))
* **deps:** bump the github-actions group with 4 updates ([74141a7](https://github.com/TerraConstructs/grid/commit/74141a7842f087f07207cdbac193877ffeb6eb71))
* **deps:** bump the github-actions group with 4 updates ([727ac34](https://github.com/TerraConstructs/grid/commit/727ac3452c5357c54c2caa737a89a2909bc0537d))
* disable restart persistence test ([44fe77f](https://github.com/TerraConstructs/grid/commit/44fe77f8981396dc21b7f9136580be4f3b9d096e))
* fix GH go cache keys ([2eba124](https://github.com/TerraConstructs/grid/commit/2eba1248bed9a25e5c00c53e24f197cee5299bb4))
* Fix renamed internal services ([791ae0d](https://github.com/TerraConstructs/grid/commit/791ae0d96c77d0518647f43809d936d3cc621d04))
* Fix sqlite failures ([baff822](https://github.com/TerraConstructs/grid/commit/baff82293c1ae1213540aa8fa4f0027fde8c9640))
* Fix webapp title ([8cf595e](https://github.com/TerraConstructs/grid/commit/8cf595ec3038209e697bd5a30e328337c0ec6a33))
* GridAPI Refactor ([62d67d2](https://github.com/TerraConstructs/grid/commit/62d67d29522ac719113998fe1edafb0c45ceac3f))
* ignore beads daemon logs ([e12b3fa](https://github.com/TerraConstructs/grid/commit/e12b3fa3202a4c22372577d55d561d94278b7183))
* refactoring and downscoping ([9cc7340](https://github.com/TerraConstructs/grid/commit/9cc734000b85358ef1e2294fe0476fa7cf86ecb5))
* run buf generate ([b0ee08f](https://github.com/TerraConstructs/grid/commit/b0ee08fdce3411c18c2c16638607e92d9a104e17))
* run buf generate ([aecc4ed](https://github.com/TerraConstructs/grid/commit/aecc4ed2cc7798a0db0209238999a026df3f3dbb))
* scope reduction ([5c3a572](https://github.com/TerraConstructs/grid/commit/5c3a57218f10083e1b22f16f1a98f5a11220ff73))
* trial Opus 4.5 speckit.plan output ([d54e098](https://github.com/TerraConstructs/grid/commit/d54e098ce7ac5877f59aa1ed01982cf1b94d25ec))
* Update SSD with beads ([6f75f4e](https://github.com/TerraConstructs/grid/commit/6f75f4e24cec709c0201ba3c23827da1e4114c92))


### Documentation

* Add comprehensive JSON Schema validation implementation plan ([e8d0f78](https://github.com/TerraConstructs/grid/commit/e8d0f78c43e8faf6874b1c17ba0a6e69045a823e))
* Add integration tests summary documentation ([107078c](https://github.com/TerraConstructs/grid/commit/107078cc3f8df809ba68ce5ccc3423722ef4d510))
* Add webapp design for output schema display ([310ddbd](https://github.com/TerraConstructs/grid/commit/310ddbde7dac8a7d19c35bb43819bd752200cb79))


### Code Refactoring

* Add bun ORM relationship annotations and eager loading methods ([70535bd](https://github.com/TerraConstructs/grid/commit/70535bdc5409d82408e6d2473bf46d7ac40e7f5a))
* Add Bun ORM relationship support to eliminate N+1 queries ([c387998](https://github.com/TerraConstructs/grid/commit/c38799845d964c25f9f2c4205ac8bb567198f4b3))
* **gridctl:** comprehensive UX improvements for consistency ([bfbfa4d](https://github.com/TerraConstructs/grid/commit/bfbfa4d1bec4a41eb7505d2f33cd3e2bd13c1ff0))
* **gridctl:** migrate to config context pattern ([752cc6a](https://github.com/TerraConstructs/grid/commit/752cc6aaee2290b66f29e505a7575c304c4fc1fd))
* use Bun dialect constants for database type checks ([7658a85](https://github.com/TerraConstructs/grid/commit/7658a85d10f00dcf276c3fcd8ba9434f921b3351))
* Use React Flow built-in arrowheads instead of custom markers ([95a559a](https://github.com/TerraConstructs/grid/commit/95a559a3b2f12266401895e2dce22822107ce480))


### Tests

* Add comprehensive integration tests for output schema feature ([a802faa](https://github.com/TerraConstructs/grid/commit/a802faa64a8049ed1c491145ec76bac5037eb8fa))
* add SQLite integration test suite ([edffcfb](https://github.com/TerraConstructs/grid/commit/edffcfbec4f8bf9fccd6c4f91c9e2c3734181008))
* update integration tests for UX changes ([d5faf21](https://github.com/TerraConstructs/grid/commit/d5faf216c38c32c305cc90246c2ad3dc9a39753b))

## Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
