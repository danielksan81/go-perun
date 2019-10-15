// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

pragma solidity 0.5.12;
pragma experimental ABIEncoderV2;
import './AssetHolder.sol';
import './SafeMath.sol';

contract AssetHolderETH is AssetHolder {

	using SafeMath for uint256;

	constructor(address _adjudicator) public {
		Adjudicator = _adjudicator;
	}

	// Deposit is used to deposit money into a channel
	// The parameter participantID = H(channelID||address)
	// This hides both the channelID as well as the participant address until a channel is settled.
	function deposit(bytes32 participantID, uint256 amount) external payable {
		require(msg.value == amount, 'Insufficent ETH for deposit');
		holdings[participantID] = holdings[participantID].add(amount);
		emit Deposited(participantID);
	}

	function withdraw(WithdrawalAuth memory authorization, bytes memory signature) public {
		require(settled[authorization.channelID], 'channel not settled');
		require(verifySignature(abi.encode(authorization), signature, authorization.participant), 'signature verification failed');
		bytes32 id = keccak256(abi.encodePacked(authorization.channelID, authorization.participant));
		require(holdings[id] >= authorization.amount, 'insufficient ETH for withdraw');
		// Decrease holdings, then transfer the money.
		holdings[id] = holdings[id].sub(authorization.amount);
		authorization.receiver.transfer(authorization.amount);
	}
}