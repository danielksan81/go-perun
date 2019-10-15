// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

pragma solidity 0.5.12;
pragma experimental ABIEncoderV2;

import "./PerunTypes.sol";
import "./ValidTransition.sol";

contract TrivialApp is ValidTransitioner {

	function validTransition(
	    PerunTypes.Params calldata params,
		PerunTypes.State calldata from,
		PerunTypes.State calldata to,
		uint256 moverIdx)
	external pure returns (bool)
	{
	    return true;
	}
}
