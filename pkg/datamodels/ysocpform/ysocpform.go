package ysocpform

// Generate OpenConfig platform GoStruct code
//go:generate generator -output_file=gen.go -compress_paths=true -path=yang -exclude_modules=ietf-interfaces,openconfig-interfaces -package_name=ysocpform -fakeroot_name=root -prefer_operational_state=true -ignore_shadow_schema_paths=true -shorten_enum_leaf_names=true -generate_fakeroot=true -include_schema=false -generate_getters=true -generate_leaf_getters=true -generate_delete=true -generate_populate_defaults=true openconfig-platform.yang openconfig-platform-transceiver.yang
