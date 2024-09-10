Welcome to the v5.0.0-beta5.63 release of Sealos!🎉🎉!



## Changelog
### New Features
* 68c0244a7369804b08e7bc1b78ceb79479063a5c: feat(costcenter): add gift code (#5026) (@HUAHUAI23)
* 80f468297e83abbf7a94b0d56a663e748d616b82: feat(desktop): Implement smart dock behavior (#4998) (@zjy365)
* 3f6343238e5787dcb6a84a1b366911441b01cb3b: feat(desktop): signup user sem data add keyword data (#4983) (@HUAHUAI23)
* dd2ac2060769d05f6ed5e48ddba14ac7b98b7eb1: feat(docs): add SEM keywords parameter (#4981) (@zjy365)
* 122350cf64c774d4a8947e5749c49db3bacb3373: feat: acmedns (@zijiren233)
* 9ca435cba8059047bf7d63c4b4c2d0ac6869c1a2: feat: add SEO TDK support and pricebox to template (#5038) (@zjy365)
* 1faf9d65714fb97d1667228c5a4449c34a77aa06: feat: add support for launching creation page in launchpad (#4984) (@zjy365)
* bab49b738cb9a23d86a398d35a45f6ab9b3e32b5: feat: applaunchpad log previous (#5047) (@zijiren233)
* 54c0180ff3e91fbef15a99ce0dad5b5e05ff7aed: feat: cost quota and price (#5014) (@zijiren233)
* cca4da935f9cdfbc0abb1651cd55720a21d4e668: feat: get os traffic from minio (#4968) (@nowinkeyy)
* 022e1a0fcf93574b2a16e637794f8cca0f232982: feat: support user external domain (#5021) (@zijiren233)
* bd3f6d45266be04f3fa10a0a86790d3b8be32b74: feat: templates support conditional rendering (#4937) (@zijiren233)
* 7091b8665cab2b149c9e2fef629f64289cb17f49: feat: user private ns invite (#5043) (@zijiren233)
* f4bb39d5a1ea10580c6f0702f8f29091db92b4be: feat:desktop update sem (#4993) (@zjy365)
### Bug fixes
* 2bf0773d44271c94b78d854d87bb14c5437194c1: fix(desktop):fix init database error (#5003) (@xudaotutou)
* 4ec113475ba8c61e1f668854b38e31efaa431d20: fix: adjust GPU quota for launchpad (#5041) (@zjy365)
* 5d6fab9f3acbdda8f2d19402e9e62f1b39685a21: fix: defaults and inputs maybe empty (#4987) (@zijiren233)
* ba66b046781a9644ad91806aa42aebf0a00cd127: fix: resolve desktop monitoring issue (#5019) (@zjy365)
* a7a7cacb95df719a6e4eedd8e8729e4ce32a493a: fix: signup with password donot save kc (#5036) (@zijiren233)
* 8eef22eb8879ae0095831133baef990fdf05e1e5: fix: switch pod logs (#5040) (@zijiren233)
* c69a4605ecdc2f9dcccd7417bf6c2f3347d7f456: fix: template undefined value (#4992) (@zijiren233)
### Build process updates
* ae37edef3f24dff0eb8698f0f04ee61f5e66debd: build: add Husky and lint-staged for code formatting (#4982) (@zjy365)
### Other work
* e3c9b629b409aeff73e61e554078430168f8ec32:  add default rbac rules for devbox runtime and runtime class. (#5012) (@lingdie)
* 17601ae99548e62fabff95f2fa069e8e1ed38f6f: 5.0.0 New document (#5032) (@bearslyricattack)
* 2882addbbc621a61843d9c76e37634058e7adb90: Feat/invoice (#4988) (@bxy4543)
* 7e6cb8bccb9e65fe9405dd8dcf432e7cd604e689: Monitor api adapt token (#5049) (@bxy4543)
* e91f227c171defe16cebe34e78125af03aac4137: Optimize/payment (#5000) (@bxy4543)
* 31471d7a572346df2ad693dcf46e40758770f752: Update costcenter (#4990) (@xudaotutou)
* 0549a9a8eec347ce2a69ff7dbb0d387180de4adc: Update db backup (#4976) (@wallyxjh)
* 3d625499da793e60124dd9dece02deef441de8c5: add additionalPrinterColumns for devbox. (#5020) (@lingdie)
* 132eb38634dbeb70afe171a27f7edfbf49d815b5: add backuprepo (#4986) (@wallyxjh)
* a42f12a9ef59bad8033f0e875a2403c6ef791955: add backuprepo (#4989) (@wallyxjh)
* 7e77b7a8795b27473f3ba1ffdfa490ad90f2f850: add devbox controller rbac, enable in ci to build docker images (#5009) (@lingdie)
* 306bbda51914f2e7ab0fa39cdf667cacb3029a39: add devbox controller rbac. (#5028) (@lingdie)
* a3c9c55ae71f258df704b0c9fdfe00a3d9eded77: add devbox controller. (#4999) (@lingdie)
* 980e796e93a14b77b5d01caeb735e6b37a646ec5: add devbox default runtime ref namespace to devbox-system. (#5034) (@lingdie)
* 4a17349ac2f65dcbd955bf339ba58d0204b67639: add devbox node and containerId (#5045) (@bearslyricattack)
* 0ed063560f8ae1898e24298a795dab6934023786: add devbox phase and controller (#5042) (@bearslyricattack)
* e52f5d3c51ac314af58c35fa4f14bed31fcfef1b: add devbox pod hostname set to devbox name (#5015) (@lingdie)
* 086681e078871b356340d41880b2cc04ead87426: add devbox proposal (#4900) (@fanux)
* d58375ebd643599dbee95cd3669dbb3260244b74: add devbox restart pod (#5010) (@bearslyricattack)
* 7f8ff72737d2defaaf8921496066b59eff8a36ff: add dify installation QA (#4991) (@wallyxjh)
* 569bcfd27035965e4375d7b8e541ee0470c5c019: add extraEnv toleration affinity,ephemeral-storage limit (#5023) (@bearslyricattack)
* 5877ddb053faa770d923db313a3f7fa129692354: add generate public and private key (#5004) (@bearslyricattack)
* 7d882d0504748d52e40c811270bb1523232742cd: add logs for CheckPodConsistency. (#5035) (@lingdie)
* e4e57023de116f259deddcba4083fbaa98b074fc: add random (#5050) (@bearslyricattack)
* d47170c6897924140ac2379b067bb698e7b7a53e: add release old tag (#5053) (@bearslyricattack)
* 60c4065bb9e4dcab4a4842c192fda7b0c62f8726: add service port (#5048) (@bearslyricattack)
* 35c1c569d8eb7b68a00bb9ba31d88ac2822037b0: change label location (#5022) (@bearslyricattack)
* 5d1d9ba83ecc96a4f2408e5cb7765b476df469dc: change runtime and runtime class to namespace scope, improve devbox c… (#5024) (@lingdie)
* 8a6844f602b8912716b39874834f236faa550bf6: feat(account-service): add use giftcode (#5013) (@HUAHUAI23)
* 47001c9ba85927dc846e13b6078ec4b9fe5f38ee: fix  release bug (#5051) (@bearslyricattack)
* 4e13ab5fd865062811b321f03b557252fbbe8356: fix devbox get runtime. (#5030) (@lingdie)
* 95ce6b8177f91fcf32275e497fa022e58b71257e: fix ssh volume bug (#5039) (@bearslyricattack)
* 3c418c33641627e33938f4d7df5566fb29527992: optimize get app cost with index(owner+order_id) (#5002) (@bxy4543)
* 11221466e78621b4cb734bc43acb124104c4c522: refactor:Fetch Launchpad pricing from service API (#5025) (@zjy365)
* fd6733eaaba0177d39b9112cd5ffb692c6d5aa58: set AutomountServiceAccountToken to false (#5001) (@lingdie)
* e8d627101ef54033fe1402224e487d913301008c: styles(objectstorage): fix styles (#4959) (@xudaotutou)
* 06caa81dc975e64d7a96a070ad5a8e756f6e3679: update cpu/mem usage (#5006) (@wallyxjh)
* 9e3327a7e6009d550c22ba91998b95264ab5e711: update cpu/mem usage (#5008) (@wallyxjh)
* d2eb60c857fea42ccc9ad65b930c756f74f73737: update devbox to add delete resource. (#5017) (@lingdie)
* b2f04454666dc5ef162c452b16008f5bc7cbf119: update launchpad cpu/mem usage (#5016) (@wallyxjh)
* 4fabfc989886d3cf3473125874b41691ea6424f7: update launchpad single (#5052) (@wallyxjh)
* 8ad51bcb2e96cb3bec3a695a1f91fe5a64b42d97: update launchpad test (#5044) (@wallyxjh)

**Full Changelog**: https://github.com/zijiren233/sealos/compare/v5.0.0-beta5.39...v5.0.0-beta5.63

See [the CHANGELOG](https://github.com/zijiren233/sealos/blob/main/CHANGELOG/CHANGELOG.md) for more details.

Your patronage towards Sealos is greatly appreciated 🎉🎉.

If you encounter any problems during its usage, please create an issue in the [GitHub repository](https://github.com/zijiren233/sealos), we're committed to resolving your problem as soon as possible.
