Welcome to the v5.0.1-beta3 release of Sealos!🎉🎉!



## Changelog
### New Features
* a884a80fcbcbc3df4789ab87f7559a48d4d2c818: feat(desktop): add face auth and enterprise auth (#5124) (@HUAHUAI23)
* f4ad702e914ef7710420c44a17b29f755429d595: feat(desktop): hidden enterprise auth (#5149) (@HUAHUAI23)
* 2b74a1281cdd72ec5f02a0cc9edf042639a1e054: feat: Implement app guide module (#5115) (@zjy365)
* ebe7f51afe023810fc4e3538f4f2e46007d43002: feat: launchpad support secret userDomains (#5119) (@zjy365)
### Bug fixes
* 6489ce6a6d7aaf2aa24ee3514bacab924b0f31a9: fix(desktop):fix merge user (#5101) (@xudaotutou)
* 88e418bca3ae926905aaeb406e809f04596568b1: fix: bool conv to str (#5136) (@zijiren233)
* 2fd2f07892660ce4fa0ad0e167082fbeb372d579: fix: cronjob arm64 image (#5113) (@zijiren233)
* d3ba17bc4925707a32ae999e88019aaf3af23800: fix: node tls reject unauthorized (#5116) (@zijiren233)
* b38d6b1cdeb93a3d94d99f7e3343f3b1ea06b93b: fix: sealos reset panic (#5147) (@yangxggo)
* bc0b5f72006a173f9bcac0e287fd9893efe728af: fix: some ui adjust and bug fix (#5097) (@mlhiter)
* 70abe3191c98bdf2dc1a2fcb0a95a9bae0d44815: fix: template deploy env bool conv to str (#5138) (@zijiren233)
* b64cbf5b2d07a75caf09934697ea875055d14ac4: fix: upgrade higress to 2.0.1 to fix higress-ca-root-cert appearing in other namespaces (#5133) (@zijiren233)
* a90ad407b2d9c42b43f95cd6e2ec05a6c0f512bb: fix:template add desktop domain (#5130) (@zjy365)
### Other work
* 7b0ffaffe61431a1873a1c55830be53805999184:  ci: cloud release add skip releases. (#5135) (@lingdie)
* 351e7616134950859fe559c57499b6fdff67fa0d: Feat/active task (#5121) (@bxy4543)
* 2e31c99ea803a53dc5dcaf2ee2538b020fa05b60: Modify the dbprovider cost estimate obtained through the account service (#5146) (@bxy4543)
* d9ff2d864ee660339d35ed72ffcce850c76b3af2: Unify devbox app-cr name. (#5145) (@zzjin)
* fdf191378bd7b145f314378fdce758512093dd28: ci: cloud release add self hosted runner. (#5131) (@lingdie)
* 9817f6fb81b163f42c7d0b31f7e6350101b0f569: devbox cache improve. (#5122) (@lingdie)
* 8c0c34592369874ce178c956af713dba3c17a9ea: devobx ignore extra ports. (#5112) (@lingdie)
* 66097b1a021f852eaaa9f07431e0edef6e46113e: docs: update links (#5128) (@ghostloda)
* d9b34cd60044590b6a5807d1725398017a735e19: fix build offline scripts and ci. (#5125) (@lingdie)
* 0704bd7f1f76e2240bfdc91b5ee67672a28e877e: fix ci (#5132) (@lingdie)
* 1243fd31e8f2f8828e35f1c47e6e6cd19155ae3e: fix desktop and costcenter configs (#5114) (@xudaotutou)
* 22d9a138f9a3f4cd04a4eb927639fd008972adff: fix devbox phase generate. (#5120) (@lingdie)
* f55786bc310f7afb2660906797aa7a9ba40b014f: fix enterprise auth translation (#5142) (@HUAHUAI23)
* b6053c1c2601fafa0752cdaac4501606bd020a20: fix get default property (#5108) (@bxy4543)
* 7b025ae80d215cd1fe860ed08bc69dc132185be3: fix init user (#5104) (@bxy4543)
* a9154f858db2b50ce72e1e0a6fc8b15a954d37a0: fix invaild (#5118) (@xudaotutou)
* d21832075623248f831df5f04b7a8962c3c83fb6: fix scripts: init account jwt secret (#5117) (@bxy4543)
* 4613b47797c8e93cb189d58deebad5e0a0c3fce0: fix the init job to create user permissions (#5153) (@bxy4543)
* ed4e7b12bd24921e777d3ba86a1f1846f5726108: fix the payment controller start error (#5150) (@bxy4543)
* 3081c5dc1abf65d751de29b52b1febeb3dbbad9a: fix(account-service): fix concurrency issue and add real name info api (#5103) (@HUAHUAI23)
* 7b0be9352fa422f1abca8f232cd8c864db548f7e: release: sealos v5.0.1 beta2 (#5110) (@lingdie)
* 98395f7dc1582013ee20a8f147573cd2227614ea: remove unused code (#5134) (@bxy4543)
* 7a4b7b91fe63b3b4747fc99d2753c1b7a409fd39: sealos v5.0.1-beta2 (#5071) (@lingdie)
* 54a3bc28684df2f8427c2ff1cc3969a0dd79c534: style(costcenter): recharge discount (#5111) (@xudaotutou)
* 1c6c05400d12b5028088f454e35c03636811bd91: style(costcenter): recharge discount (#5129) (@xudaotutou)
* b78691a56704884d1da0544a2779b8f42b45d3f1: sync resourcequota objectstorage/size status used (#5102) (@nowinkeyy)
* aff50d3e31e640c205fb1e628309b1c66bd2b542: v5.0.1 docs. (#5126) (@lingdie)
* c80c265fd6838bbf071cefcfa077a4e4d0274fa4: 🤖 add release changelog using rebot. (#5100) (@sealos-release-robot)
* 56ffcb829d61b2d5531fe4bb544e17bbe12032ba: 🤖 add release changelog using rebot. (#5123) (@sealos-release-robot)
* 04311fd74eafc33bb7804b2d05923a59a8723512: 🤖 add release changelog using rebot. (#5127) (@sealos-release-robot)

**Full Changelog**: https://github.com/zijiren233/sealos/compare/v5.0.1-beta1...v5.0.1-beta3

See [the CHANGELOG](https://github.com/zijiren233/sealos/blob/main/CHANGELOG/CHANGELOG.md) for more details.

Your patronage towards Sealos is greatly appreciated 🎉🎉.

If you encounter any problems during its usage, please create an issue in the [GitHub repository](https://github.com/zijiren233/sealos), we're committed to resolving your problem as soon as possible.
