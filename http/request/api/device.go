package api

// DeviceCliForm 对应客户端 `/api/devices/cli` 的 body（rustdesk-cli 注册设备属性）。
// 字段名与 RustDesk 客户端 core_main.rs 里 serde_json 构造完全一致。
type DeviceCliForm struct {
	Id                  string `json:"id"`
	Uuid                string `json:"uuid"`
	UserName            string `json:"user_name"`
	StrategyName        string `json:"strategy_name"`
	AddressBookName     string `json:"address_book_name"`
	AddressBookTag      string `json:"address_book_tag"`
	AddressBookAlias    string `json:"address_book_alias"`
	AddressBookPassword string `json:"address_book_password"`
	AddressBookNote     string `json:"address_book_note"`
	DeviceGroupName     string `json:"device_group_name"`
	Note                string `json:"note"`
	DeviceUsername      string `json:"device_username"`
	DeviceName          string `json:"device_name"`
}

// DeviceDeployForm 对应客户端 `/api/devices/deploy` 的 body（rustdesk-cli 部署设备）。
type DeviceDeployForm struct {
	Id   string `json:"id"`
	Uuid string `json:"uuid"`
	Pk   string `json:"pk"`
}
