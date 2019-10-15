// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

pragma solidity 0.5.12;
pragma experimental ABIEncoderV2;

import "./PerunTypes.sol";
import "./ValidTransition.sol";
import "./AssetHolder.sol";
import "./SafeMath.sol";
import "./ECDSA.sol";

contract Adjudicator {

	using SafeMath for uint256;

	enum DisputeState { DISPUTE, FORCEMOVE }

	// Mapping channelID => H(parameters, state, timeout).
	mapping(bytes32 => bytes32) public disputeRegistry;

	// Events used by the contract.
	event Registered(bytes32 indexed channelID, uint256 version);
	event Refuted(bytes32 indexed channelID, uint256 version);
	event Responded(bytes32 indexed channelID, uint256 version);
	event Stored(bytes32 indexed channelID, uint256 timeout);
	event FinalStateRegistered(bytes32 indexed channelID);
	event Concluded(bytes32 indexed channelID);
	event PushOutcome(bytes32 indexed channelID);

	// Restricts functions to only be called before a certain timeout.
	modifier beforeTimeout(uint256 timeout)
	{
		require(now < timeout, 'function called after timeout');
		_;
	}

	// Restricts functions to only be called after a certain timeout.
	modifier afterTimeout(uint256 timeout)
	{
		require(now >= timeout, 'function called before timeout');
		_;
	}

	// Register registers a non-final state of a channel.
	// It can only be called if no other dispute is currently in progress.
	// The caller has to provide n signatures on the state.
	// If the call was sucessful a Registered event is emitted.
	function register(
		PerunTypes.Params memory p,
		PerunTypes.State memory s,
		bytes[] memory sigs)
	public
	{
		bytes32 channelID = calculateChannelID(p);
		require(s.channelID == channelID, 'tried registering invalid channelID');
		require(disputeRegistry[channelID] == bytes32(0), 'a dispute was already registered');
		validateSignatures(p, s, sigs);
		storeChallenge(p, s, channelID, DisputeState.DISPUTE);
		emit Registered(channelID, s.version);
	}

	// Refute is called to refute a dispute.
	// It can only be called with a higher state.
	// The caller has to provide n signatures on the new state.
	// If the call was sucessful a Refuted event is emitted.
	function refute(
		PerunTypes.Params memory p,
		PerunTypes.State memory old,
		uint256 timeout,
		PerunTypes.State memory s,
		bytes[] memory sigs)
	public beforeTimeout(timeout)
	{
		require(s.version > old.version, 'only a refutation with a newer state is valid');
		bytes32 channelID = calculateChannelID(p);
		require(s.channelID == channelID, 'tried refutation with invalid channelID');
		require(disputeRegistry[channelID] == hashDispute(p, old, timeout, DisputeState.DISPUTE),
			'provided wrong old state/timeout');
		validateSignatures(p, s, sigs);
		storeChallenge(p, s, channelID, DisputeState.DISPUTE);
		emit Refuted(channelID, s.version);
	}

	// Progress is used to advance the state of an app on-chain.
	// It corresponds to the force-move functionality of magmo.
	// The caller only has to provide a valid signature from the mover.
	// This method can only advance the state by one.
	// If the call was successful, a Responded event is emitted.
	function progress(
		PerunTypes.Params memory p,
		PerunTypes.State memory old,
		uint256 timeout,
		DisputeState disputeState,
		PerunTypes.State memory s,
		uint256 moverIdx,
		bytes memory sig)
	public
	{
		if(disputeState == DisputeState.DISPUTE) {
			require(now >= timeout, 'function called before timeout');
		}

		bytes32 channelID = calculateChannelID(p);
		require(s.channelID == channelID, 'tried to respond with invalid channelID');
		require(disputeRegistry[channelID] == hashDispute(p, old, timeout, disputeState), 'provided wrong old state/timeout');
		address signer = recoverSigner(s, sig);
		require(moverIdx < p.participants.length);
		require(p.participants[moverIdx] == signer, 'moverIdx is not set to the id of the sender');
		validTransition(p, old, s, moverIdx);
		storeChallenge(p, s, channelID, DisputeState.FORCEMOVE);
		emit Responded(channelID, s.version);
	}

	// ConcludeChallenge is used to finalize a channel on-chain.
	// It can only be called after the timeout is over.
	// If the call was successful, a Concluded event is emitted.
	function concludeChallenge(
		PerunTypes.Params memory p,
		PerunTypes.State memory s,
		uint256 timeout,
		DisputeState disputeState)
	public afterTimeout(timeout)
	{
		bytes32 channelID = calculateChannelID(p);
		require(disputeRegistry[channelID] == hashDispute(p, s, timeout, disputeState), 'provided wrong old state/timeout');
		pushOutcome(channelID, p, s);
		emit Concluded(channelID);
	}

	// RegisterFinalState can be used to register a final state.
	// The caller has to provide n signatures on a finalized state.
	// It can only be called, if no other dispute was registered.
	// If the call was successful, a FinalStateRegistered event is emitted.
	function registerFinalState(
		PerunTypes.Params memory p,
		PerunTypes.State memory s,
		bytes[] memory sigs)
	public
	{
		require(s.isFinal == true, 'only accept final states');
		bytes32 channelID = calculateChannelID(p);
		require(s.channelID == channelID, 'tried registering invalid channelID');
		require(disputeRegistry[channelID] == bytes32(0), 'a dispute was already registered');
		validateSignatures(p, s, sigs);
		pushOutcome(channelID, p, s);
		emit FinalStateRegistered(channelID);
	}

	function calculateChannelID(PerunTypes.Params memory p) internal pure returns (bytes32) {
		return keccak256(abi.encode(p.challengeDuration, p.nonce, p.app, p.participants));
	}

	function storeChallenge(
		PerunTypes.Params memory p,
		PerunTypes.State memory s,
		bytes32 channelID,
		DisputeState disputeState)
	internal
	{
		uint256 timeout = now.add(p.challengeDuration);
		disputeRegistry[channelID] = hashDispute(p, s, timeout, disputeState);
		emit Stored(channelID, timeout);
	}

	function hashDispute(
		PerunTypes.Params memory p,
		PerunTypes.State memory s,
		uint256 timeout,
		DisputeState disputeState)
	internal pure returns (bytes32)
	{
		return keccak256(abi.encode(p, s, timeout, uint256(disputeState)));
	}

	function validTransition(
		PerunTypes.Params memory p,
		PerunTypes.State memory old,
		PerunTypes.State memory s,
		uint256 moverIdx)
	internal pure
	{
		require(s.version == old.version + 1, 'can only advance the version counter by one');
		checkAssetPreservation(old.outcome, s.outcome, p.participants.length);
		ValidTransitioner va = ValidTransitioner(p.app);
		require(va.validTransition(p, old, s, moverIdx), 'invalid new state');
	}

	function checkAssetPreservation(
		PerunTypes.Allocation memory oldAlloc,
		PerunTypes.Allocation memory newAlloc,
		uint256 participantsLength)
	internal pure
	{
		assert(oldAlloc.balances.length == newAlloc.balances.length);
		assert(oldAlloc.assets.length == newAlloc.assets.length);
		for (uint256 i = 0; i < newAlloc.assets.length; i++) {
			require(oldAlloc.assets[i] == newAlloc.assets[i], 'asset addresses mismatch');
			uint256 sumOld = 0;
			uint256 sumNew = 0;
			assert(oldAlloc.balances[i].length == newAlloc.balances[i].length);
			assert(oldAlloc.balances[i].length == participantsLength);
			for (uint256 k = 0; k < newAlloc.balances[i].length; k++) {
				sumOld = sumOld.add(oldAlloc.balances[i][k]);
				sumNew = sumNew.add(newAlloc.balances[i][k]);
			}
			// Add the sums of all subAllocs
			for (uint256 k = 0; k < oldAlloc.locked.length; k++) {
				sumOld = sumOld.add(oldAlloc.locked[k].balances[i]);
				sumNew = sumNew.add(newAlloc.locked[k].balances[i]);
			}
			require(sumOld == sumNew, 'Sum of balances for an asset must be equal');
		}
	}


	function pushOutcome(
		bytes32 channelID,
		PerunTypes.Params memory p,
		PerunTypes.State memory s)
	internal
	{
		uint256[][] memory balances = new uint256[][](s.outcome.assets.length);
		bytes32[] memory subAllocs = new bytes32[](s.outcome.locked.length);
		// Iterate over all subAllocations
		for(uint256 k = 0; k < s.outcome.locked.length; k++) {
			subAllocs[k] = s.outcome.locked[k].ID;
			// Iterate over all Assets
			for(uint256 i = 0; i < s.outcome.assets.length; i++) {
				// init subarrays
				if (k == 0)
					balances[i] = new uint256[](s.outcome.locked.length);
				balances[i][k] = balances[i][k].add(s.outcome.locked[k].balances[i]);
			}
		}

		for (uint256 i = 0; i < s.outcome.assets.length; i++) {
			AssetHolder a = AssetHolder(s.outcome.assets[i]);
			require(s.outcome.balances[i].length == p.participants.length, 'balances length should match participants length');
			a.setOutcome(channelID, p.participants, s.outcome.balances[i], subAllocs, balances[i]);
		}
		emit PushOutcome(channelID);
	}

	function validateSignatures(
		PerunTypes.Params memory p,
		PerunTypes.State memory s,
		bytes[] memory sigs)
	internal pure
	{
		assert(p.participants.length == sigs.length);
		for (uint256 i = 0; i < sigs.length; i++) {
			address signer = recoverSigner(s, sigs[i]);
			require(p.participants[i] == signer, 'invalid signature');
		}
	}

	function recoverSigner(
		PerunTypes.State memory s,
		bytes memory sig)
	internal pure returns (address)
	{
		bytes memory prefix = '\x19Ethereum Signed Message:\n32';
		bytes memory subAlloc = abi.encode(s.outcome.locked[0].ID, s.outcome.locked[0].balances);
		bytes memory outcome = abi.encode(s.outcome.assets, s.outcome.balances, subAlloc);
		bytes memory state = abi.encode(s.channelID, s.version, outcome, s.appData, s.isFinal);
		bytes32 h = keccak256(state);
		bytes32 prefixedHash = keccak256(abi.encodePacked(prefix, h));
		address recoveredAddr = ECDSA.recover(prefixedHash, sig);
		require(recoveredAddr != address(0));
		return recoveredAddr;
	}

}