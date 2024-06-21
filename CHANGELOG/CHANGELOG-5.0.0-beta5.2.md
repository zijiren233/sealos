Welcome to the v5.0.0-beta5.2 release of Sealos!🎉🎉!



## Changelog
### New Features
* e19143794992d526fcd0775f455b1157f0081a4e: feat(applaunchpad): support file browser for pv (#4674) (@0fatal)
* 18464891fca36fd623da4f17faff2a035343dfa6: feat(desktop):add inviting others into workspace by link (#4712) (@xudaotutou)
* 835e22457b23e9be7700a11734d41c694231818a: feat: add providers workorder app  (#4718) (@zjy365)
* d4040594a3cd148bf832651d435cf68d501db689: feat: cloud host supports GPU (#4728) (@zjy365)
* 4b2d3c0e72851ff5573d0c85f32450aa526df7e3: feat: desktop support controlling the size of the evoked window (#4700) (@zjy365)
* 3859de964955dda9a350ddbc5cda7f9f48c2cc6a: feat: upgrade cost center config. (#4756) (@lingdie)
* f0e17bc07ea8b512cd0895022e3176425b607b3c: feat:add cloudserver app  (#4698) (@zjy365)
### Bug fixes
* add7316542a417b8ab0b1452c76873be60b38b50: fix(costcenter): remove bank account regex (#4713) (@xudaotutou)
* 9c178471c7e72b5beb2a9736a20b45fb4fda0fe4: fix(costcenter): update amount by the query (#4737) (@xudaotutou)
* 950e6cb44eb03f6e097a501a80b160eefe9017fc: fix: Delete unnecessary logs (#4710) (@ghostloda)
* 18be6fe4d903f3982900f07c80bea40853c9a109: fix:add check containerd (#4748) (@ghostloda)
* 5968bae3f03db7bf520294c497ca20264b0253d2: fix:cloudserver bandwidth price display  (#4734) (@zjy365)
* 7b5fdf221c342f7e65611a5cd706dca78fa53bbb: fix:db podAntiAffinity Preferred (#4701) (@zjy365)
* bca847623a2c821c184d42fcf165c6fc5b8059e6: fix:launchpad checkNetworkPorts (#4724) (@zjy365)
* c723346d6ef16d9cc32feec91c46e4043c14eb1f: fix:launchpad config yaml boolean (#4715) (@zjy365)
### Documentation updates
* 29fb771de5298272254b6e04e593cc2e6f4e4e2d: doc(Frontend): Rename READMD.md to README.md (#4755) (@luoling8192)
### Other work
* b4187c53b61f384ca21d36e4b4b3d1de7688b35a: Add clear mongo log (#4757) (@wallyxjh)
* f04556bed1fe47332d9ad8286f308c4e9bb5db63: Change object storage monitor service from prometheus to vm (#4727) (@nowinkeyy)
* 1ad3c7185c5a88f9ddf3d7fdad96e141ffd24c62: Feat/ add vms/email debt notification (#4763) (@bxy4543)
* 8cb7b734000b94dc59e0b9a9722c1fbc466562e5: Feat: Cloud Virtual Machine Billing (#4699) (@bxy4543)
* afb2584ddcebf4d4c1655209704d1647152866e1: Fix desktop config (#4733) (@lingdie)
* 4d02f7661760c2a5fc140697bd95867bf14ea35e: Fix desktop password salt (#4762) (@lingdie)
* 2138f4656ed70f2d9c4a14ccdc8c7ce1a9ab4c0d: Fix doc link 404. (#4743) (@zzjin)
* 49e6422e89b7a11a47631e62c8291a5df870b0d9: Fix kubepanel typo&link. (#4729) (@zzjin)
* 13bd347c7e33097b304a2f159f08e79a6d517d0b: Fix/costcente svc (#4736) (@bxy4543)
* 3b044f328db58acdd0381c0aec7809027e2d2025: Launchpad monitor (#4689) (@wallyxjh)
* 16c8524f7b05201aea5d3edcbd10bc41be3827ab: Stop clusters (#4644) (@wallyxjh)
* 372632d16ee8f06110399b00c4a7a7e54e31fcce: Update README.md (#4745) (@fanux)
* a7c1d9df0e86ea64fb54860f661b4c52a6ffec16: Update app menu types. (#4735) (@zzjin)
* 17c2b43ac55b1170a236e043d3ed007e0bb8aa14: Update appcr (#4742) (@lingdie)
* c0bb1e72dd2564edef48e3b38d7c84719c76d6c4: add a key for a bucket (#4714) (@nowinkeyy)
* 7362d807e9c1612f97f1b9151a6701090431e338: change mongo 4.0 to 4.4 (#4708) (@lingdie)
* 47ba1b87d3adec8bbd48b1d1f557a36d1d65172b: docs: Update sealos version in self-hosting docs (#4683) (@yangchuansheng)
* daff4ac6f63a3f04938ef813c2d387b848b5aef7: feat. change desktop to use config file. (#4709) (@lingdie)
* c56ce7aebd2dd7eab3fdcb45764b513fdbd6ebbb: fix billing record query with app type (#4731) (@bxy4543)
* 786985e3cced44b14d8c1faa1018d208db671883: fix control-plane component status (#4749) (@ghostloda)
* dd1cfc9221c260fa2e4f0f9d344775f9aaeeb7bc: fix cost center frontend manifests (#4761) (@lingdie)
* 1d0ad5c5213e4915f196f7a9348a33c1e9cb65d2: fix object storage monitor (#4691) (@bxy4543)
* f6b1e76230978671d5ff52be4eb6d1cc232ccedc: fix objectstorage frontend deploy.yaml format error (#4725) (@nowinkeyy)
* 26739d9a6cee1afa95bae74e63c21a4f1b57ec5c: fix set account create region id (#4739) (@bxy4543)
* 9a4f321c1d19d9d0e58dd1569455f1b41316c091: i18n: update i18n for App Launchpad (#4754) (@yangchuansheng)
* 4c3fac74d54699dac47d1e029547b34fce601c00: monitor svc multi nodePort (#4726) (@bxy4543)
* 4b80930b9090afc7384ef6d6b875743a186c4d97: none (#4732) (@nowinkeyy)
* 9bb4c44653a379e2e36e21a5e3f9b700d13c974d: optimize query object storage metric (#4686) (@nowinkeyy)
* a05b86025cc091977f2b7ab53853d42b641cdb05: refactor(costcenter): refactor invoice (#4694) (@xudaotutou)
* 2fe621674f37a307d8af515ac7d316b3f6650065: style: launchpad displays monitoring values (#4688) (@zjy365)
* 0e1693c32ac098a0b78d793ab334a98246f03d10: styles: Updated UI design for dbprovider frontend (#4747) (@zjy365)
* a58a5b159dcfe74381ce5e85abff9f95149a8eca: styles: launchpad AppStatusTag & workorder appendixs (#4740) (@zjy365)
* f164e9dd48e8a8f6b4ac229dac96e7036854c29a: temporary hiding suspends kb package import (#4753) (@bxy4543)
* 7974ba511348f0e1a1acb472e694c460744a9cb4: 为kubepanel network 添加service (#4705) (@bearslyricattack)
* 4535a7fcaceeeb9de1b61018e013abf3674db0a0: 🤖 add release changelog using rebot. (#4681) (@sealos-release-robot)

**Full Changelog**: https://github.com/zijiren233/sealos/compare/v5.0.0-beta5.1...v5.0.0-beta5.2

See [the CHANGELOG](https://github.com/zijiren233/sealos/blob/main/CHANGELOG/CHANGELOG.md) for more details.

Your patronage towards Sealos is greatly appreciated 🎉🎉.

If you encounter any problems during its usage, please create an issue in the [GitHub repository](https://github.com/zijiren233/sealos), we're committed to resolving your problem as soon as possible.
