package vechain

//上链成功观察者
type IObserver interface {
	//上链数据的处理
	Execute(hash, vid, txid string) error
}
