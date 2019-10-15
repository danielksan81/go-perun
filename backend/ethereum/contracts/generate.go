package contracts

//go:generate echo -e "\\e[01;31mEnsure that solc version is 0.5.11\\e[0m"
//go:generate solc --bin-runtime --optimize Adjudicator.sol --overwrite -o ./
//go:generate solc --bin-runtime --optimize AssetHolderETH.sol --overwrite -o ./
//go:generate ./abigen --pkg adjudicator --sol Adjudicator.sol --out ../adjudicator/Adjudicator.go
//go:generate ./abigen --pkg funder --sol AssetHolderETH.sol --out ../funder/AssetHolderETH.go
//go:generate ./abigen --version
//go:generate echo "package contract" > ../adjudicator/Adjudicator.go
//go:generate echo -e "\\e[01;31mDeleting dublicated structs in ../adjudicator/Adjudicator.go\\e[0m"
//go:generate python script.py
