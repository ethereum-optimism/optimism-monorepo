// SPDX-License-Identifier: MIT
// @unsupported: ovm
pragma solidity >0.5.0 <0.8.0;
pragma experimental ABIEncoderV2;

/* Library Imports */
import { Lib_OVMCodec } from "../../libraries/codec/Lib_OVMCodec.sol";
import { Lib_AddressResolver } from "../../libraries/resolver/Lib_AddressResolver.sol";
import { Lib_Bytes32Utils } from "../../libraries/utils/Lib_Bytes32Utils.sol";
import { Lib_EthUtils } from "../../libraries/utils/Lib_EthUtils.sol";
import { Lib_ErrorUtils } from "../../libraries/utils/Lib_ErrorUtils.sol";
import { Lib_PredeployAddresses } from "../../libraries/constants/Lib_PredeployAddresses.sol";

/* Interface Imports */
import { iOVM_ExecutionManager } from "../../iOVM/execution/iOVM_ExecutionManager.sol";
import { iOVM_StateManager } from "../../iOVM/execution/iOVM_StateManager.sol";

/* Contract Imports */
import { OVM_DeployerWhitelist } from "../predeploys/OVM_DeployerWhitelist.sol";
import { OVM_SafetyChecker } from "../predeploys/OVM_SafetyChecker.sol";

/* External Imports */
import { Math } from "@openzeppelin/contracts/math/Math.sol";

/**
 * @title OVM_ExecutionManager
 * @dev The Execution Manager (EM) is the core of our OVM implementation, and provides a sandboxed
 * environment allowing us to execute OVM transactions deterministically on either Layer 1 or
 * Layer 2.
 * The EM's run() function is the first function called during the execution of any
 * transaction on L2.
 * For each context-dependent EVM operation the EM has a function which implements a corresponding
 * OVM operation, which will read state from the State Manager contract.
 * The EM relies on the Safety Checker to verify that code deployed to Layer 2 does not contain any
 * context-dependent operations.
 *
 * Compiler used: solc
 * Runtime target: EVM
 */
contract OVM_ExecutionManager is iOVM_ExecutionManager, Lib_AddressResolver {

    /********************************
     * External Contract References *
     ********************************/

    iOVM_StateManager internal ovmStateManager;


    /*******************************
     * Execution Context Variables *
     *******************************/

    GasMeterConfig internal gasMeterConfig;
    GlobalContext internal globalContext;
    TransactionContext internal transactionContext;
    MessageContext internal messageContext;
    TransactionRecord internal transactionRecord;
    MessageRecord internal messageRecord;


    /**************************
     * Gas Metering Constants *
     **************************/

    address constant GAS_METADATA_ADDRESS = 0x06a506A506a506A506a506a506A506A506A506A5;
    uint256 constant NUISANCE_GAS_SLOAD = 20000;
    uint256 constant NUISANCE_GAS_SSTORE = 20000;
    uint256 constant MIN_NUISANCE_GAS_PER_CONTRACT = 30000;
    uint256 constant NUISANCE_GAS_PER_CONTRACT_BYTE = 100;
    uint256 constant MIN_GAS_FOR_INVALID_STATE_ACCESS = 30000;


    /**************************
     * Native Value Constants *
     **************************/

    // Public so we can access and make assertions in integration tests.
    uint256 public constant CALL_WITH_VALUE_INTRINSIC_GAS = 90000;


    /**************************
     * Default Context Values *
     **************************/

    uint256 constant DEFAULT_UINT256 = 0xdefa017defa017defa017defa017defa017defa017defa017defa017defa017d;
    address constant DEFAULT_ADDRESS = 0xdEfa017defA017DeFA017DEfa017DeFA017DeFa0;


    /*************************************
     * Container Contract Address Prefix *
     *************************************/

    /**
     * @dev The Execution Manager and State Manager each have this 30 byte prefix, and are uncallable.
     */
    address constant CONTAINER_CONTRACT_PREFIX = 0xDeadDeAddeAddEAddeadDEaDDEAdDeaDDeAD0000;


    /***************
     * Constructor *
     ***************/

    /**
     * @param _libAddressManager Address of the Address Manager.
     */
    constructor(
        address _libAddressManager,
        GasMeterConfig memory _gasMeterConfig,
        GlobalContext memory _globalContext
    )
        Lib_AddressResolver(_libAddressManager)
    {
        gasMeterConfig = _gasMeterConfig;
        globalContext = _globalContext;
        _resetContext();
    }


    /**********************
     * Function Modifiers *
     **********************/

    /**
     * Applies dynamically-sized refund to a transaction to account for the difference in execution
     * between L1 and L2, so that the overall cost of the ovmOPCODE is fixed.
     * @param _cost Desired gas cost for the function after the refund.
     */
    modifier netGasCost(
        uint256 _cost
    ) {
        uint256 gasProvided = gasleft();
        _;
        uint256 gasUsed = gasProvided - gasleft();

        // We want to refund everything *except* the specified cost.
        if (_cost < gasUsed) {
            transactionRecord.ovmGasRefund += gasUsed - _cost;
        }
    }

    /**
     * Applies a fixed-size gas refund to a transaction to account for the difference in execution
     * between L1 and L2, so that the overall cost of an ovmOPCODE can be lowered.
     * @param _discount Amount of gas cost to refund for the ovmOPCODE.
     */
    modifier fixedGasDiscount(
        uint256 _discount
    ) {
        uint256 gasProvided = gasleft();
        _;
        uint256 gasUsed = gasProvided - gasleft();

        // We want to refund the specified _discount, unless this risks underflow.
        if (_discount < gasUsed) {
            transactionRecord.ovmGasRefund += _discount;
        } else {
            // refund all we can without risking underflow.
            transactionRecord.ovmGasRefund += gasUsed;
        }
    }

    /**
     * Makes sure we're not inside a static context.
     */
    modifier notStatic() {
        if (messageContext.isStatic == true) {
            _revertWithFlag(RevertFlag.STATIC_VIOLATION);
        }
        _;
    }


    /************************************
     * Transaction Execution Entrypoint *
     ************************************/

    /**
     * Starts the execution of a transaction via the OVM_ExecutionManager.
     * @param _transaction Transaction data to be executed.
     * @param _ovmStateManager iOVM_StateManager implementation providing account state.
     */
    function run(
        Lib_OVMCodec.Transaction memory _transaction,
        address _ovmStateManager
    )
        override
        external
        returns (
            bytes memory
        )
    {
        // Make sure that run() is not re-enterable.  This condition should always be satisfied
        // Once run has been called once, due to the behavior of _isValidInput().
        if (transactionContext.ovmNUMBER != DEFAULT_UINT256) {
            return bytes("");
        }

        // Store our OVM_StateManager instance (significantly easier than attempting to pass the
        // address around in calldata).
        ovmStateManager = iOVM_StateManager(_ovmStateManager);

        // Make sure this function can't be called by anyone except the owner of the
        // OVM_StateManager (expected to be an OVM_StateTransitioner). We can revert here because
        // this would make the `run` itself invalid.
        require(
            // This method may return false during fraud proofs, but always returns true in L2 nodes' State Manager precompile.
            ovmStateManager.isAuthenticated(msg.sender),
            "Only authenticated addresses in ovmStateManager can call this function"
        );

        // Initialize the execution context, must be initialized before we perform any gas metering
        // or we'll throw a nuisance gas error.
        _initContext(_transaction);

        // TEMPORARY: Gas metering is disabled for minnet.
        // // Check whether we need to start a new epoch, do so if necessary.
        // _checkNeedsNewEpoch(_transaction.timestamp);

        // Make sure the transaction's gas limit is valid. We don't revert here because we reserve
        // reverts for INVALID_STATE_ACCESS.
        if (_isValidInput(_transaction) == false) {
            _resetContext();
            return bytes("");
        }

        // TEMPORARY: Gas metering is disabled for minnet.
        // // Check gas right before the call to get total gas consumed by OVM transaction.
        // uint256 gasProvided = gasleft();

        // Run the transaction, make sure to meter the gas usage.
        (, bytes memory returndata) = ovmCALL(
            _transaction.gasLimit - gasMeterConfig.minTransactionGasLimit,
            _transaction.entrypoint,
            0,
            _transaction.data
        );

        // TEMPORARY: Gas metering is disabled for minnet.
        // // Update the cumulative gas based on the amount of gas used.
        // uint256 gasUsed = gasProvided - gasleft();
        // _updateCumulativeGas(gasUsed, _transaction.l1QueueOrigin);

        // Wipe the execution context.
        _resetContext();

        return returndata;
    }


    /******************************
     * Opcodes: Execution Context *
     ******************************/

    /**
     * @notice Overrides CALLER.
     * @return _CALLER Address of the CALLER within the current message context.
     */
    function ovmCALLER()
        override
        external
        view
        returns (
            address _CALLER
        )
    {
        return messageContext.ovmCALLER;
    }

    /**
     * @notice Overrides ADDRESS.
     * @return _ADDRESS Active ADDRESS within the current message context.
     */
    function ovmADDRESS()
        override
        public
        view
        returns (
            address _ADDRESS
        )
    {
        return messageContext.ovmADDRESS;
    }

    /**
     * @notice Overrides CALLVALUE.
     * @return _CALLVALUE Value sent along with the call according to the current message context.
     */
    function ovmCALLVALUE()
        override
        public
        view
        returns (
            uint256 _CALLVALUE
        )
    {
        return messageContext.ovmCALLVALUE;
    }

    /**
     * @notice Overrides TIMESTAMP.
     * @return _TIMESTAMP Value of the TIMESTAMP within the transaction context.
     */
    function ovmTIMESTAMP()
        override
        external
        view
        returns (
            uint256 _TIMESTAMP
        )
    {
        return transactionContext.ovmTIMESTAMP;
    }

    /**
     * @notice Overrides NUMBER.
     * @return _NUMBER Value of the NUMBER within the transaction context.
     */
    function ovmNUMBER()
        override
        external
        view
        returns (
            uint256 _NUMBER
        )
    {
        return transactionContext.ovmNUMBER;
    }

    /**
     * @notice Overrides GASLIMIT.
     * @return _GASLIMIT Value of the block's GASLIMIT within the transaction context.
     */
    function ovmGASLIMIT()
        override
        external
        view
        returns (
            uint256 _GASLIMIT
        )
    {
        return transactionContext.ovmGASLIMIT;
    }

    /**
     * @notice Overrides CHAINID.
     * @return _CHAINID Value of the chain's CHAINID within the global context.
     */
    function ovmCHAINID()
        override
        external
        view
        returns (
            uint256 _CHAINID
        )
    {
        return globalContext.ovmCHAINID;
    }

    /*********************************
     * Opcodes: L2 Execution Context *
     *********************************/

    /**
     * @notice Specifies from which source (Sequencer or Queue) this transaction originated from.
     * @return _queueOrigin Enum indicating the ovmL1QUEUEORIGIN within the current message context.
     */
    function ovmL1QUEUEORIGIN()
        override
        external
        view
        returns (
            Lib_OVMCodec.QueueOrigin _queueOrigin
        )
    {
        return transactionContext.ovmL1QUEUEORIGIN;
    }

    /**
     * @notice Specifies which L1 account, if any, sent this transaction by calling enqueue().
     * @return _l1TxOrigin Address of the account which sent the tx into L2 from L1.
     */
    function ovmL1TXORIGIN()
        override
        external
        view
        returns (
            address _l1TxOrigin
        )
    {
        return transactionContext.ovmL1TXORIGIN;
    }

    /********************
     * Opcodes: Halting *
     ********************/

    /**
     * @notice Overrides REVERT.
     * @param _data Bytes data to pass along with the REVERT.
     */
    function ovmREVERT(
        bytes memory _data
    )
        override
        public
    {
        _revertWithFlag(RevertFlag.INTENTIONAL_REVERT, _data);
    }


    /******************************
     * Opcodes: Contract Creation *
     ******************************/

    /**
     * @notice Overrides CREATE.
     * @param _bytecode Code to be used to CREATE a new contract.
     * @return Address of the created contract.
     * @return Revert data, if and only if the creation threw an exception.
     */
    function ovmCREATE(
        bytes memory _bytecode
    )
        override
        public
        notStatic
        fixedGasDiscount(40000)
        returns (
            address,
            bytes memory
        )
    {
        // Creator is always the current ADDRESS.
        address creator = ovmADDRESS();

        // Check that the deployer is whitelisted, or
        // that arbitrary contract deployment has been enabled.
        _checkDeployerAllowed(creator);

        // Generate the correct CREATE address.
        address contractAddress = Lib_EthUtils.getAddressForCREATE(
            creator,
            _getAccountNonce(creator)
        );

        return _createContract(
            contractAddress,
            _bytecode,
            MessageType.ovmCREATE
        );
    }

    /**
     * @notice Overrides CREATE2.
     * @param _bytecode Code to be used to CREATE2 a new contract.
     * @param _salt Value used to determine the contract's address.
     * @return Address of the created contract.
     * @return Revert data, if and only if the creation threw an exception.
     */
    function ovmCREATE2(
        bytes memory _bytecode,
        bytes32 _salt
    )
        override
        external
        notStatic
        fixedGasDiscount(40000)
        returns (
            address,
            bytes memory
        )
    {
        // Creator is always the current ADDRESS.
        address creator = ovmADDRESS();

        // Check that the deployer is whitelisted, or
        // that arbitrary contract deployment has been enabled.
        _checkDeployerAllowed(creator);

        // Generate the correct CREATE2 address.
        address contractAddress = Lib_EthUtils.getAddressForCREATE2(
            creator,
            _bytecode,
            _salt
        );

        return _createContract(
            contractAddress,
            _bytecode,
            MessageType.ovmCREATE2
        );
    }


    /*******************************
     * Account Abstraction Opcodes *
     ******************************/

    /**
     * Retrieves the nonce of the current ovmADDRESS.
     * @return _nonce Nonce of the current contract.
     */
    function ovmGETNONCE()
        override
        external
        returns (
            uint256 _nonce
        )
    {
        return _getAccountNonce(ovmADDRESS());
    }

    /**
     * Bumps the nonce of the current ovmADDRESS by one.
     */
    function ovmINCREMENTNONCE()
        override
        external
        notStatic
    {
        address account = ovmADDRESS();
        uint256 nonce = _getAccountNonce(account);

        // Prevent overflow.
        if (nonce + 1 > nonce) {
            _setAccountNonce(account, nonce + 1);
        }
    }

    /**
     * Creates a new EOA contract account, for account abstraction.
     * @dev Essentially functions like ovmCREATE or ovmCREATE2, but we can bypass a lot of checks
     *      because the contract we're creating is trusted (no need to do safety checking or to
     *      handle unexpected reverts). Doesn't need to return an address because the address is
     *      assumed to be the user's actual address.
     * @param _messageHash Hash of a message signed by some user, for verification.
     * @param _v Signature `v` parameter.
     * @param _r Signature `r` parameter.
     * @param _s Signature `s` parameter.
     */
    function ovmCREATEEOA(
        bytes32 _messageHash,
        uint8 _v,
        bytes32 _r,
        bytes32 _s
    )
        override
        public
        notStatic
    {
        // Recover the EOA address from the message hash and signature parameters. Since we do the
        // hashing in advance, we don't have handle different message hashing schemes. Even if this
        // function were to return the wrong address (rather than explicitly returning the zero
        // address), the rest of the transaction would simply fail (since there's no EOA account to
        // actually execute the transaction).
        address eoa = ecrecover(
            _messageHash,
            _v + 27,
            _r,
            _s
        );

        // Invalid signature is a case we proactively handle with a revert. We could alternatively
        // have this function return a `success` boolean, but this is just easier.
        if (eoa == address(0)) {
            ovmREVERT(bytes("Signature provided for EOA contract creation is invalid."));
        }

        // If the user already has an EOA account, then there's no need to perform this operation.
        if (_hasEmptyAccount(eoa) == false) {
            return;
        }

        // We always need to initialize the contract with the default account values.
        _initPendingAccount(eoa);

        // Temporarily set the current address so it's easier to access on L2.
        address prevADDRESS = messageContext.ovmADDRESS;
        messageContext.ovmADDRESS = eoa;

        // Creates a duplicate of the OVM_ProxyEOA located at 0x42....09. Uses the following
        // "magic" prefix to deploy an exact copy of the code:
        // PUSH1 0x0D   # size of this prefix in bytes
        // CODESIZE
        // SUB          # subtract prefix size from codesize
        // DUP1
        // PUSH1 0x0D
        // PUSH1 0x00
        // CODECOPY     # copy everything after prefix into memory at pos 0
        // PUSH1 0x00
        // RETURN       # return the copied code
        address proxyEOA = Lib_EthUtils.createContract(abi.encodePacked(
            hex"600D380380600D6000396000f3",
            ovmEXTCODECOPY(
                Lib_PredeployAddresses.PROXY_EOA,
                0,
                ovmEXTCODESIZE(Lib_PredeployAddresses.PROXY_EOA)
            )
        ));

        // Reset the address now that we're done deploying.
        messageContext.ovmADDRESS = prevADDRESS;

        // Commit the account with its final values.
        _commitPendingAccount(
            eoa,
            address(proxyEOA),
            keccak256(Lib_EthUtils.getCode(address(proxyEOA)))
        );

        _setAccountNonce(eoa, 0);
    }


    /*********************************
     * Opcodes: Contract Interaction *
     *********************************/

    /**
     * @notice Overrides CALL.
     * @param _gasLimit Amount of gas to be passed into this call.
     * @param _address Address of the contract to call.
     * @param _value ETH value to pass with the call.
     * @param _calldata Data to send along with the call.
     * @return _success Whether or not the call returned (rather than reverted).
     * @return _returndata Data returned by the call.
     */
    function ovmCALL(
        uint256 _gasLimit,
        address _address,
        uint256 _value,
        bytes memory _calldata
    )
        override
        public
        fixedGasDiscount(100000)
        returns (
            bool _success,
            bytes memory _returndata
        )
    {
        // CALL updates the CALLER and ADDRESS.
        MessageContext memory nextMessageContext = messageContext;
        nextMessageContext.ovmCALLER = nextMessageContext.ovmADDRESS;
        nextMessageContext.ovmADDRESS = _address;
        nextMessageContext.ovmCALLVALUE = _value;

        return _callContract(
            nextMessageContext,
            _gasLimit,
            _address,
            _calldata,
            MessageType.ovmCALL
        );
    }

    /**
     * @notice Overrides STATICCALL.
     * @param _gasLimit Amount of gas to be passed into this call.
     * @param _address Address of the contract to call.
     * @param _calldata Data to send along with the call.
     * @return _success Whether or not the call returned (rather than reverted).
     * @return _returndata Data returned by the call.
     */
    function ovmSTATICCALL(
        uint256 _gasLimit,
        address _address,
        bytes memory _calldata
    )
        override
        public
        fixedGasDiscount(80000)
        returns (
            bool _success,
            bytes memory _returndata
        )
    {
        // STATICCALL updates the CALLER, updates the ADDRESS, and runs in a static, valueless context.
        MessageContext memory nextMessageContext = messageContext;
        nextMessageContext.ovmCALLER = nextMessageContext.ovmADDRESS;
        nextMessageContext.ovmADDRESS = _address;
        nextMessageContext.isStatic = true;
        nextMessageContext.ovmCALLVALUE = 0;

        return _callContract(
            nextMessageContext,
            _gasLimit,
            _address,
            _calldata,
            MessageType.ovmSTATICCALL
        );
    }

    /**
     * @notice Overrides DELEGATECALL.
     * @param _gasLimit Amount of gas to be passed into this call.
     * @param _address Address of the contract to call.
     * @param _calldata Data to send along with the call.
     * @return _success Whether or not the call returned (rather than reverted).
     * @return _returndata Data returned by the call.
     */
    function ovmDELEGATECALL(
        uint256 _gasLimit,
        address _address,
        bytes memory _calldata
    )
        override
        public
        fixedGasDiscount(40000)
        returns (
            bool _success,
            bytes memory _returndata
        )
    {
        // DELEGATECALL does not change anything about the message context.
        MessageContext memory nextMessageContext = messageContext;

        return _callContract(
            nextMessageContext,
            _gasLimit,
            _address,
            _calldata,
            MessageType.ovmDELEGATECALL
        );
    }

    /**
     * @notice Legacy ovmCALL function which did not support ETH value; this maintains backwards compatibility.
     * @param _gasLimit Amount of gas to be passed into this call.
     * @param _address Address of the contract to call.
     * @param _calldata Data to send along with the call.
     * @return _success Whether or not the call returned (rather than reverted).
     * @return _returndata Data returned by the call.
     */
    function ovmCALL(
        uint256 _gasLimit,
        address _address,
        bytes memory _calldata
    )
        override
        public
        returns(
            bool _success,
            bytes memory _returndata
        )
    {
        // Legacy ovmCALL assumed always-0 value.
        return ovmCALL(
            _gasLimit,
            _address,
            0,
            _calldata
        );
    }


    /************************************
     * Opcodes: Contract Storage Access *
     ************************************/

    /**
     * @notice Overrides SLOAD.
     * @param _key 32 byte key of the storage slot to load.
     * @return _value 32 byte value of the requested storage slot.
     */
    function ovmSLOAD(
        bytes32 _key
    )
        override
        external
        netGasCost(40000)
        returns (
            bytes32 _value
        )
    {
        // We always SLOAD from the storage of ADDRESS.
        address contractAddress = ovmADDRESS();

        return _getContractStorage(
            contractAddress,
            _key
        );
    }

    /**
     * @notice Overrides SSTORE.
     * @param _key 32 byte key of the storage slot to set.
     * @param _value 32 byte value for the storage slot.
     */
    function ovmSSTORE(
        bytes32 _key,
        bytes32 _value
    )
        override
        external
        notStatic
        netGasCost(60000)
    {
        // We always SSTORE to the storage of ADDRESS.
        address contractAddress = ovmADDRESS();

        _putContractStorage(
            contractAddress,
            _key,
            _value
        );
    }


    /*********************************
     * Opcodes: Contract Code Access *
     *********************************/

    /**
     * @notice Overrides EXTCODECOPY.
     * @param _contract Address of the contract to copy code from.
     * @param _offset Offset in bytes from the start of contract code to copy beyond.
     * @param _length Total number of bytes to copy from the contract's code.
     * @return _code Bytes of code copied from the requested contract.
     */
    function ovmEXTCODECOPY(
        address _contract,
        uint256 _offset,
        uint256 _length
    )
        override
        public
        returns (
            bytes memory _code
        )
    {
        return Lib_EthUtils.getCode(
            _getAccountEthAddress(_contract),
            _offset,
            _length
        );
    }

    /**
     * @notice Overrides EXTCODESIZE.
     * @param _contract Address of the contract to query the size of.
     * @return _EXTCODESIZE Size of the requested contract in bytes.
     */
    function ovmEXTCODESIZE(
        address _contract
    )
        override
        public
        returns (
            uint256 _EXTCODESIZE
        )
    {
        return Lib_EthUtils.getCodeSize(
            _getAccountEthAddress(_contract)
        );
    }

    /**
     * @notice Overrides EXTCODEHASH.
     * @param _contract Address of the contract to query the hash of.
     * @return _EXTCODEHASH Hash of the requested contract.
     */
    function ovmEXTCODEHASH(
        address _contract
    )
        override
        external
        returns (
            bytes32 _EXTCODEHASH
        )
    {
        return Lib_EthUtils.getCodeHash(
            _getAccountEthAddress(_contract)
        );
    }


    /***************************************
     * Public Functions: ETH Value Opcodes *
     ***************************************/

    /**
     * @notice Overrides BALANCE.
     * NOTE: In the future, this could be optimized to directly invoke EM._getContractStorage(...).
     * @param _contract Address of the contract to query the OVM_ETH balance of.
     * @return _BALANCE OVM_ETH balance of the requested contract.
     */
    function ovmBALANCE(
        address _contract
    )
        override
        public
        returns (
            uint256 _BALANCE
        )
    {
        // Easiest way to get the balance is query OVM_ETH as normal.
        bytes memory balanceOfCalldata = abi.encodeWithSignature(
            "balanceOf(address)",
            _contract
        );

        // Static call because this should be a read-only query.
        (bool success, bytes memory returndata) = ovmSTATICCALL(
            gasleft(),
            Lib_PredeployAddresses.OVM_ETH,
            balanceOfCalldata
        );

        // All balanceOf queries should successfully return a uint, otherwise this must be an OOG.
        if (!success || returndata.length != 32) {
            _revertWithFlag(RevertFlag.OUT_OF_GAS);
        }

        // Return the decoded balance.
        return abi.decode(returndata, (uint256));
    }

    /**
     * @notice Overrides SELFBALANCE.
     * @return _BALANCE OVM_ETH balance of the requesting contract.
     */
    function ovmSELFBALANCE()
        override
        external
        returns (
            uint256 _BALANCE
        )
    {
        return ovmBALANCE(ovmADDRESS());
    }


    /***************************************
     * Public Functions: Execution Context *
     ***************************************/

    function getMaxTransactionGasLimit()
        external
        view
        override
        returns (
            uint256 _maxTransactionGasLimit
        )
    {
        return gasMeterConfig.maxTransactionGasLimit;
    }

    /*****************************************
     * Internal Functions: Deployment Checks *
     *****************************************/

    /**
     * Checks whether the given address is on the whitelist to ovmCREATE/ovmCREATE2, and reverts if not.
     * @param _deployerAddress Address attempting to deploy a contract.
     */
    function _checkDeployerAllowed(
        address _deployerAddress
    )
        internal
    {
        // From an OVM semantics perspective, this will appear identical to
        // the deployer ovmCALLing the whitelist.  This is fine--in a sense, we are forcing them to.
        (bool success, bytes memory data) = ovmSTATICCALL(
            gasleft(),
            Lib_PredeployAddresses.DEPLOYER_WHITELIST,
            abi.encodeWithSelector(
                OVM_DeployerWhitelist.isDeployerAllowed.selector,
                _deployerAddress
            )
        );
        bool isAllowed = abi.decode(data, (bool));

        if (!isAllowed || !success) {
            _revertWithFlag(RevertFlag.CREATOR_NOT_ALLOWED);
        }
    }

    /**
     * Checks whether the given the given bytecode is safe to run in the OVM
     * @param _code Init or runtime bytecode.
     */
    function _checkBytecodeSafety(
        bytes memory _code
    )
        internal
        returns(
            bool
        )
    {
        // From an OVM semantics perspective, this will appear identical to
        // the deployer ovmCALLing the SafetyCache.  This is fine--in a sense, we are forcing them to.
        (bool success, bytes memory data) = ovmCALL(
            gasleft(),
            0x420000000000000000000000000000000000000c,
            abi.encodeWithSelector(
                OVM_SafetyChecker.checkAndRegisterSafeBytecode.selector,
                _code
            )
        );

        if (!success) {
            return false;
        }

        // decodes to true if and only if safe
        return abi.decode(data, (bool));
    }


    /********************************************
     * Internal Functions: Contract Interaction *
     ********************************************/

    /**
     * Creates a new contract and associates it with some contract address.
     * @param _contractAddress Address to associate the created contract with.
     * @param _bytecode Bytecode to be used to create the contract.
     * @return Final OVM contract address.
     * @return Revertdata, if and only if the creation threw an exception.
     */
    function _createContract(
        address _contractAddress,
        bytes memory _bytecode,
        MessageType _messageType
    )
        internal
        returns (
            address,
            bytes memory
        )
    {
        // We always update the nonce of the creating account, even if the creation fails.
        _setAccountNonce(ovmADDRESS(), _getAccountNonce(ovmADDRESS()) + 1);

        // We're stepping into a CREATE or CREATE2, so we need to update ADDRESS to point
        // to the contract's associated address and CALLER to point to the previous ADDRESS.
        MessageContext memory nextMessageContext = messageContext;
        nextMessageContext.ovmCALLER = messageContext.ovmADDRESS;
        nextMessageContext.ovmADDRESS = _contractAddress;

        // Run the common logic which occurs between call-type and create-type messages,
        // passing in the creation bytecode and `true` to trigger create-specific logic.
        (bool success, bytes memory data) = _handleExternalMessage(
            nextMessageContext,
            gasleft(),
            _contractAddress,
            _bytecode,
            _messageType
        );

        // Yellow paper requires that address returned is zero if the contract deployment fails.
        return (
            success ? _contractAddress : address(0),
            data
        );
    }

    /**
     * Calls the deployed contract associated with a given address.
     * @param _nextMessageContext Message context to be used for the call.
     * @param _gasLimit Amount of gas to be passed into this call.
     * @param _contract OVM address to be called.
     * @param _calldata Data to send along with the call.
     * @return _success Whether or not the call returned (rather than reverted).
     * @return _returndata Data returned by the call.
     */
    function _callContract(
        MessageContext memory _nextMessageContext,
        uint256 _gasLimit,
        address _contract,
        bytes memory _calldata,
        MessageType _messageType
    )
        internal
        returns (
            bool _success,
            bytes memory _returndata
        )
    {
        // We reserve addresses of the form 0xdeaddeaddead...NNNN for the container contracts in L2 geth.
        // So, we block calls to these addresses since they are not safe to run as an OVM contract itself.
        if (
            (uint256(_contract) & uint256(0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0000))
            == uint256(CONTAINER_CONTRACT_PREFIX)
        ) {
            // EVM does not return data in the success case, see: https://github.com/ethereum/go-ethereum/blob/aae7660410f0ef90279e14afaaf2f429fdc2a186/core/vm/instructions.go#L600-L604
            return (true, hex'');
        }

        // Both 0x0000... and the EVM precompiles have the same address on L1 and L2 --> no trie lookup needed.
        address codeContractAddress =
            uint(_contract) < 100
            ? _contract
            : _getAccountEthAddress(_contract);

        return _handleExternalMessage(
            _nextMessageContext,
            _gasLimit,
            codeContractAddress,
            _calldata,
            _messageType
        );
    }

    /**
     * Handles all interactions which involve the execution manager calling out to untrusted code (both calls and creates).
     * Ensures that OVM-related measures are enforced, including L2 gas refunds, nuisance gas, and flagged reversions.
     *
     * @param _nextMessageContext Message context to be used for the external message.
     * @param _gasLimit Amount of gas to be passed into this message. NOTE: this argument is overwritten in some cases to avoid stack-too-deep.
     * @param _contract OVM address being called or deployed to
     * @param _data Data for the message (either calldata or creation code)
     * @param _messageType What type of ovmOPCODE this message corresponds to.
     * @return Whether or not the message (either a call or deployment) succeeded.
     * @return Data returned by the message.
     */
    function _handleExternalMessage(
        MessageContext memory _nextMessageContext,
        // NOTE: this argument is overwritten in some cases to avoid stack-too-deep.
        uint256 _gasLimit,
        address _contract,
        bytes memory _data,
        MessageType _messageType
    )
        internal
        returns (
            bool,
            bytes memory
        )
    {
        uint256 messageValue = _nextMessageContext.ovmCALLVALUE;
        // If there is value in this message, we need to transfer the ETH over before switching contexts.
        if (
            messageValue > 0
            && _isValueType(_messageType)
        ) {
            // Handle out-of-intrinsic gas consistent with EVM behavior -- the subcall "appears to revert" if we don't have enough gas to transfer the ETH.
            // Similar to dynamic gas cost of value exceeding gas here:
            // https://github.com/ethereum/go-ethereum/blob/c503f98f6d5e80e079c1d8a3601d188af2a899da/core/vm/interpreter.go#L268-L273
            if (gasleft() < CALL_WITH_VALUE_INTRINSIC_GAS) {
                return (false, hex"");
            }

            // If there *is* enough gas to transfer ETH, then we need to make sure this amount of gas is reserved (i.e. not
            // given to the _contract.call below) to guarantee that _handleExternalMessage can't run out of gas.
            // In particular, in the event that the call fails, we will need to transfer the ETH back to the sender.
            // Taking the lesser of _gasLimit and gasleft() - CALL_WITH_VALUE_INTRINSIC_GAS guarantees that the second
            // _attemptForcedEthTransfer below, if needed, always has enough gas to succeed.
            _gasLimit = Math.min(
                _gasLimit,
                gasleft() - CALL_WITH_VALUE_INTRINSIC_GAS // Cannot overflow due to the above check.
            );

            // Now transfer the value of the call.
            // The target is interpreted to be the next message's ovmADDRESS account.
            bool transferredOvmEth = _attemptForcedEthTransfer(
                _nextMessageContext.ovmADDRESS,
                messageValue
            );

            // If the ETH transfer fails (should only be possible in the case of insufficient balance), then treat this as a revert.
            // This mirrors EVM behavior, see https://github.com/ethereum/go-ethereum/blob/2dee31930c9977af2a9fcb518fb9838aa609a7cf/core/vm/evm.go#L298
            if (!transferredOvmEth) {
                return (false, hex"");
            }
        }

        // We need to switch over to our next message context for the duration of this call.
        MessageContext memory prevMessageContext = messageContext;
        _switchMessageContext(prevMessageContext, _nextMessageContext);

        // Nuisance gas is a system used to bound the ability for an attacker to make fraud proofs
        // expensive by touching a lot of different accounts or storage slots. Since most contracts
        // only use a few storage slots during any given transaction, this shouldn't be a limiting
        // factor.
        uint256 prevNuisanceGasLeft = messageRecord.nuisanceGasLeft;
        uint256 nuisanceGasLimit = _getNuisanceGasLimit(_gasLimit);
        messageRecord.nuisanceGasLeft = nuisanceGasLimit;

        // Make the call and make sure to pass in the gas limit. Another instance of hidden
        // complexity. `_contract` is guaranteed to be a safe contract, meaning its return/revert
        // behavior can be controlled. In particular, we enforce that flags are passed through
        // revert data as to retrieve execution metadata that would normally be reverted out of
        // existence.
        bool success;
        bytes memory returndata;
        if (_isCreateType(_messageType)) {
            // safeCREATE() is a function which replicates a CREATE message, but uses return values
            // Which match that of CALL (i.e. bool, bytes).  This allows many security checks to be
            // to be shared between untrusted call and create call frames.
            (success, returndata) = address(this).call{gas: _gasLimit}(
                abi.encodeWithSelector(
                    this.safeCREATE.selector,
                    _data,
                    _contract
                )
            );
        } else {
            (success, returndata) = _contract.call{gas: _gasLimit}(_data);
        }

        // If the message threw an exception, its value should be returned back to the sender.
        // So, we force it back, BEFORE returning the messageContext to the previous addresses.
        // This operation is part of the reason we "reserved the intrinsic gas" above.
        if (
            messageValue > 0
            && _isValueType(_messageType)
            && !success
        ) {
            bool transferredOvmEth = _attemptForcedEthTransfer(
                prevMessageContext.ovmADDRESS,
                messageValue
            );

            // Since we transferred it in above and the call reverted, the transfer back should always pass.
            // This code path should NEVER be triggered since we sent `messageValue` worth of OVM_ETH into the target
            // and reserved sufficient gas to execute the transfer, but in case there is some edge case which has
            // been missed, we revert the entire frame (and its parent) to make sure the ETH gets sent back.
            if (!transferredOvmEth) {
                _revertWithFlag(RevertFlag.OUT_OF_GAS);
            }
        }

        // Switch back to the original message context now that we're out of the call and all OVM_ETH is in the right place.
        _switchMessageContext(_nextMessageContext, prevMessageContext);

        // Assuming there were no reverts, the message record should be accurate here. We'll update
        // this value in the case of a revert.
        uint256 nuisanceGasLeft = messageRecord.nuisanceGasLeft;

        // Reverts at this point are completely OK, but we need to make a few updates based on the
        // information passed through the revert.
        if (success == false) {
            (
                RevertFlag flag,
                uint256 nuisanceGasLeftPostRevert,
                uint256 ovmGasRefund,
                bytes memory returndataFromFlag
            ) = _decodeRevertData(returndata);
            // INVALID_STATE_ACCESS is the only flag that triggers an immediate abort of the
            // parent EVM message. This behavior is necessary because INVALID_STATE_ACCESS must
            // halt any further transaction execution that could impact the execution result.
            if (flag == RevertFlag.INVALID_STATE_ACCESS) {
                _revertWithFlag(flag);
            }

            // INTENTIONAL_REVERT, UNSAFE_BYTECODE, STATIC_VIOLATION, and CREATOR_NOT_ALLOWED aren't
            // dependent on the input state, so we can just handle them like standard reverts. Our only change here
            // is to record the gas refund reported by the call (enforced by safety checking).
            if (
                flag == RevertFlag.INTENTIONAL_REVERT
                || flag == RevertFlag.UNSAFE_BYTECODE
                || flag == RevertFlag.STATIC_VIOLATION
                || flag == RevertFlag.CREATOR_NOT_ALLOWED
            ) {
                transactionRecord.ovmGasRefund = ovmGasRefund;
            }

            // INTENTIONAL_REVERT needs to pass up the user-provided return data encoded into the
            // flag, *not* the full encoded flag.  Additionally, we surface custom error messages
            // to developers in the case of unsafe creations for improved devex.
            // All other revert types return no data.
            if (
                flag == RevertFlag.INTENTIONAL_REVERT
                || flag == RevertFlag.UNSAFE_BYTECODE
            ) {
                returndata = returndataFromFlag;
            } else {
                returndata = hex'';
            }

            // Reverts mean we need to use up whatever "nuisance gas" was used by the call.
            // EXCEEDS_NUISANCE_GAS explicitly reduces the remaining nuisance gas for this message
            // to zero. OUT_OF_GAS is a "pseudo" flag given that messages return no data when they
            // run out of gas, so we have to treat this like EXCEEDS_NUISANCE_GAS. All other flags
            // will simply pass up the remaining nuisance gas.
            nuisanceGasLeft = nuisanceGasLeftPostRevert;
        }

        // We need to reset the nuisance gas back to its original value minus the amount used here.
        messageRecord.nuisanceGasLeft = prevNuisanceGasLeft - (nuisanceGasLimit - nuisanceGasLeft);

        return (
            success,
            returndata
        );
    }

    /**
     * Handles the creation-specific safety measures required for OVM contract deployment.
     * This function sanitizes the return types for creation messages to match calls (bool, bytes),
     * by being an external function which the EM can call, that mimics the success/fail case of the CREATE.
     * This allows for consistent handling of both types of messages in _handleExternalMessage().
     * Having this step occur as a separate call frame also allows us to easily revert the
     * contract deployment in the event that the code is unsafe.
     *
     * @param _creationCode Code to pass into CREATE for deployment.
     * @param _address OVM address being deployed to.
     */
    function safeCREATE(
        bytes memory _creationCode,
        address _address
    )
        external
    {
        // The only way this should callable is from within _createContract(),
        // and it should DEFINITELY not be callable by a non-EM code contract.
        if (msg.sender != address(this)) {
            return;
        }
        // Check that there is not already code at this address.
        if (_hasEmptyAccount(_address) == false) {
            // Note: in the EVM, this case burns all allotted gas.  For improved
            // developer experience, we do return the remaining gas.
            _revertWithFlag(
                RevertFlag.CREATE_COLLISION
            );
        }

        // Check the creation bytecode against the OVM_SafetyChecker
        if (_checkBytecodeSafety(_creationCode) == false) {
            // Note: in the EVM, this case burns all allotted gas.  For improved
            // developer experience, we do return the remaining gas.
            _revertWithFlag(
                RevertFlag.UNSAFE_BYTECODE,
                Lib_ErrorUtils.encodeRevertString("Contract creation code contains unsafe opcodes. Did you use the right compiler or pass an unsafe constructor argument?")
            );
        }

        // We always need to initialize the contract with the default account values.
        _initPendingAccount(_address);

        // Actually execute the EVM create message.
        // NOTE: The inline assembly below means we can NOT make any evm calls between here and then.
        address ethAddress = Lib_EthUtils.createContract(_creationCode);

        if (ethAddress == address(0)) {
            // If the creation fails, the EVM lets us grab its revert data. This may contain a revert flag
            // to be used above in _handleExternalMessage, so we pass the revert data back up unmodified.
            assembly {
                returndatacopy(0,0,returndatasize())
                revert(0, returndatasize())
            }
        }

        // Again simply checking that the deployed code is safe too. Contracts can generate
        // arbitrary deployment code, so there's no easy way to analyze this beforehand.
        bytes memory deployedCode = Lib_EthUtils.getCode(ethAddress);
        if (_checkBytecodeSafety(deployedCode) == false) {
            _revertWithFlag(
                RevertFlag.UNSAFE_BYTECODE,
                Lib_ErrorUtils.encodeRevertString("Constructor attempted to deploy unsafe bytecode.")
            );
        }

        // Contract creation didn't need to be reverted and the bytecode is safe. We finish up by
        // associating the desired address with the newly created contract's code hash and address.
        _commitPendingAccount(
            _address,
            ethAddress,
            Lib_EthUtils.getCodeHash(ethAddress)
        );
    }

    /******************************************
     * Internal Functions: Value Manipulation *
     ******************************************/

    /**
     * Invokes an ovmCALL to OVM_ETH.transfer on behalf of the current ovmADDRESS, allowing us to force movement of OVM_ETH in correspondence with ETH's native value functionality.
     * WARNING: this will send on behalf of whatever the messageContext.ovmADDRESS is in storage at the time of the call.
     * NOTE: In the future, this could be optimized to directly invoke EM._setContractStorage(...).
     * @param _to Amount of OVM_ETH to be sent.
     * @param _value Amount of OVM_ETH to send.
     * @return _success Whether or not the transfer worked.
     */
    function _attemptForcedEthTransfer(
        address _to,
        uint256 _value
    )
        internal
        returns(
            bool _success
        )
    {
        bytes memory transferCalldata = abi.encodeWithSignature(
            "transfer(address,uint256)",
            _to,
            _value
        );

        // OVM_ETH inherits from the UniswapV2ERC20 standard.  In this implementation, its return type
        // is a boolean.  However, the implementation always returns true if it does not revert.
        // Thus, success of the call frame is sufficient to infer success of the transfer itself.
        (bool success, ) = ovmCALL(
            gasleft(),
            Lib_PredeployAddresses.OVM_ETH,
            0,
            transferCalldata
        );

        return success;
    }

    /******************************************
     * Internal Functions: State Manipulation *
     ******************************************/

    /**
     * Checks whether an account exists within the OVM_StateManager.
     * @param _address Address of the account to check.
     * @return _exists Whether or not the account exists.
     */
    function _hasAccount(
        address _address
    )
        internal
        returns (
            bool _exists
        )
    {
        _checkAccountLoad(_address);
        return ovmStateManager.hasAccount(_address);
    }

    /**
     * Checks whether a known empty account exists within the OVM_StateManager.
     * @param _address Address of the account to check.
     * @return _exists Whether or not the account empty exists.
     */
    function _hasEmptyAccount(
        address _address
    )
        internal
        returns (
            bool _exists
        )
    {
        _checkAccountLoad(_address);
        return ovmStateManager.hasEmptyAccount(_address);
    }

    /**
     * Sets the nonce of an account.
     * @param _address Address of the account to modify.
     * @param _nonce New account nonce.
     */
    function _setAccountNonce(
        address _address,
        uint256 _nonce
    )
        internal
    {
        _checkAccountChange(_address);
        ovmStateManager.setAccountNonce(_address, _nonce);
    }

    /**
     * Gets the nonce of an account.
     * @param _address Address of the account to access.
     * @return _nonce Nonce of the account.
     */
    function _getAccountNonce(
        address _address
    )
        internal
        returns (
            uint256 _nonce
        )
    {
        _checkAccountLoad(_address);
        return ovmStateManager.getAccountNonce(_address);
    }

    /**
     * Retrieves the Ethereum address of an account.
     * @param _address Address of the account to access.
     * @return _ethAddress Corresponding Ethereum address.
     */
    function _getAccountEthAddress(
        address _address
    )
        internal
        returns (
            address _ethAddress
        )
    {
        _checkAccountLoad(_address);
        return ovmStateManager.getAccountEthAddress(_address);
    }

    /**
     * Creates the default account object for the given address.
     * @param _address Address of the account create.
     */
    function _initPendingAccount(
        address _address
    )
        internal
    {
        // Although it seems like `_checkAccountChange` would be more appropriate here, we don't
        // actually consider an account "changed" until it's inserted into the state (in this case
        // by `_commitPendingAccount`).
        _checkAccountLoad(_address);
        ovmStateManager.initPendingAccount(_address);
    }

    /**
     * Stores additional relevant data for a new account, thereby "committing" it to the state.
     * This function is only called during `ovmCREATE` and `ovmCREATE2` after a successful contract
     * creation.
     * @param _address Address of the account to commit.
     * @param _ethAddress Address of the associated deployed contract.
     * @param _codeHash Hash of the code stored at the address.
     */
    function _commitPendingAccount(
        address _address,
        address _ethAddress,
        bytes32 _codeHash
    )
        internal
    {
        _checkAccountChange(_address);
        ovmStateManager.commitPendingAccount(
            _address,
            _ethAddress,
            _codeHash
        );
    }

    /**
     * Retrieves the value of a storage slot.
     * @param _contract Address of the contract to query.
     * @param _key 32 byte key of the storage slot.
     * @return _value 32 byte storage slot value.
     */
    function _getContractStorage(
        address _contract,
        bytes32 _key
    )
        internal
        returns (
            bytes32 _value
        )
    {
        _checkContractStorageLoad(_contract, _key);
        return ovmStateManager.getContractStorage(_contract, _key);
    }

    /**
     * Sets the value of a storage slot.
     * @param _contract Address of the contract to modify.
     * @param _key 32 byte key of the storage slot.
     * @param _value 32 byte storage slot value.
     */
    function _putContractStorage(
        address _contract,
        bytes32 _key,
        bytes32 _value
    )
        internal
    {
        // We don't set storage if the value didn't change. Although this acts as a convenient
        // optimization, it's also necessary to avoid the case in which a contract with no storage
        // attempts to store the value "0" at any key. Putting this value (and therefore requiring
        // that the value be committed into the storage trie after execution) would incorrectly
        // modify the storage root.
        if (_getContractStorage(_contract, _key) == _value) {
            return;
        }

        _checkContractStorageChange(_contract, _key);
        ovmStateManager.putContractStorage(_contract, _key, _value);
    }

    /**
     * Validation whenever a contract needs to be loaded. Checks that the account exists, charges
     * nuisance gas if the account hasn't been loaded before.
     * @param _address Address of the account to load.
     */
    function _checkAccountLoad(
        address _address
    )
        internal
    {
        // See `_checkContractStorageLoad` for more information.
        if (gasleft() < MIN_GAS_FOR_INVALID_STATE_ACCESS) {
            _revertWithFlag(RevertFlag.OUT_OF_GAS);
        }

        // See `_checkContractStorageLoad` for more information.
        if (ovmStateManager.hasAccount(_address) == false) {
            _revertWithFlag(RevertFlag.INVALID_STATE_ACCESS);
        }

        // Check whether the account has been loaded before and mark it as loaded if not. We need
        // this because "nuisance gas" only applies to the first time that an account is loaded.
        (
            bool _wasAccountAlreadyLoaded
        ) = ovmStateManager.testAndSetAccountLoaded(_address);

        // If we hadn't already loaded the account, then we'll need to charge "nuisance gas" based
        // on the size of the contract code.
        if (_wasAccountAlreadyLoaded == false) {
            _useNuisanceGas(
                (Lib_EthUtils.getCodeSize(_getAccountEthAddress(_address)) * NUISANCE_GAS_PER_CONTRACT_BYTE) + MIN_NUISANCE_GAS_PER_CONTRACT
            );
        }
    }

    /**
     * Validation whenever a contract needs to be changed. Checks that the account exists, charges
     * nuisance gas if the account hasn't been changed before.
     * @param _address Address of the account to change.
     */
    function _checkAccountChange(
        address _address
    )
        internal
    {
        // Start by checking for a load as we only want to charge nuisance gas proportional to
        // contract size once.
        _checkAccountLoad(_address);

        // Check whether the account has been changed before and mark it as changed if not. We need
        // this because "nuisance gas" only applies to the first time that an account is changed.
        (
            bool _wasAccountAlreadyChanged
        ) = ovmStateManager.testAndSetAccountChanged(_address);

        // If we hadn't already loaded the account, then we'll need to charge "nuisance gas" based
        // on the size of the contract code.
        if (_wasAccountAlreadyChanged == false) {
            ovmStateManager.incrementTotalUncommittedAccounts();
            _useNuisanceGas(
                (Lib_EthUtils.getCodeSize(_getAccountEthAddress(_address)) * NUISANCE_GAS_PER_CONTRACT_BYTE) + MIN_NUISANCE_GAS_PER_CONTRACT
            );
        }
    }

    /**
     * Validation whenever a slot needs to be loaded. Checks that the account exists, charges
     * nuisance gas if the slot hasn't been loaded before.
     * @param _contract Address of the account to load from.
     * @param _key 32 byte key to load.
     */
    function _checkContractStorageLoad(
        address _contract,
        bytes32 _key
    )
        internal
    {
        // Another case of hidden complexity. If we didn't enforce this requirement, then a
        // contract could pass in just enough gas to cause the INVALID_STATE_ACCESS check to fail
        // on L1 but not on L2. A contract could use this behavior to prevent the
        // OVM_ExecutionManager from detecting an invalid state access. Reverting with OUT_OF_GAS
        // allows us to also charge for the full message nuisance gas, because you deserve that for
        // trying to break the contract in this way.
        if (gasleft() < MIN_GAS_FOR_INVALID_STATE_ACCESS) {
            _revertWithFlag(RevertFlag.OUT_OF_GAS);
        }

        // We need to make sure that the transaction isn't trying to access storage that hasn't
        // been provided to the OVM_StateManager. We'll immediately abort if this is the case.
        // We know that we have enough gas to do this check because of the above test.
        if (ovmStateManager.hasContractStorage(_contract, _key) == false) {
            _revertWithFlag(RevertFlag.INVALID_STATE_ACCESS);
        }

        // Check whether the slot has been loaded before and mark it as loaded if not. We need
        // this because "nuisance gas" only applies to the first time that a slot is loaded.
        (
            bool _wasContractStorageAlreadyLoaded
        ) = ovmStateManager.testAndSetContractStorageLoaded(_contract, _key);

        // If we hadn't already loaded the account, then we'll need to charge some fixed amount of
        // "nuisance gas".
        if (_wasContractStorageAlreadyLoaded == false) {
            _useNuisanceGas(NUISANCE_GAS_SLOAD);
        }
    }

    /**
     * Validation whenever a slot needs to be changed. Checks that the account exists, charges
     * nuisance gas if the slot hasn't been changed before.
     * @param _contract Address of the account to change.
     * @param _key 32 byte key to change.
     */
    function _checkContractStorageChange(
        address _contract,
        bytes32 _key
    )
        internal
    {
        // Start by checking for load to make sure we have the storage slot and that we charge the
        // "nuisance gas" necessary to prove the storage slot state.
        _checkContractStorageLoad(_contract, _key);

        // Check whether the slot has been changed before and mark it as changed if not. We need
        // this because "nuisance gas" only applies to the first time that a slot is changed.
        (
            bool _wasContractStorageAlreadyChanged
        ) = ovmStateManager.testAndSetContractStorageChanged(_contract, _key);

        // If we hadn't already changed the account, then we'll need to charge some fixed amount of
        // "nuisance gas".
        if (_wasContractStorageAlreadyChanged == false) {
            // Changing a storage slot means that we're also going to have to change the
            // corresponding account, so do an account change check.
            _checkAccountChange(_contract);

            ovmStateManager.incrementTotalUncommittedContractStorage();
            _useNuisanceGas(NUISANCE_GAS_SSTORE);
        }
    }


    /************************************
     * Internal Functions: Revert Logic *
     ************************************/

    /**
     * Simple encoding for revert data.
     * @param _flag Flag to revert with.
     * @param _data Additional user-provided revert data.
     * @return _revertdata Encoded revert data.
     */
    function _encodeRevertData(
        RevertFlag _flag,
        bytes memory _data
    )
        internal
        view
        returns (
            bytes memory _revertdata
        )
    {
        // Out of gas and create exceptions will fundamentally return no data, so simulating it shouldn't either.
        if (
            _flag == RevertFlag.OUT_OF_GAS
        ) {
            return bytes('');
        }

        // INVALID_STATE_ACCESS doesn't need to return any data other than the flag.
        if (_flag == RevertFlag.INVALID_STATE_ACCESS) {
            return abi.encode(
                _flag,
                0,
                0,
                bytes('')
            );
        }

        // Just ABI encode the rest of the parameters.
        return abi.encode(
            _flag,
            messageRecord.nuisanceGasLeft,
            transactionRecord.ovmGasRefund,
            _data
        );
    }

    /**
     * Simple decoding for revert data.
     * @param _revertdata Revert data to decode.
     * @return _flag Flag used to revert.
     * @return _nuisanceGasLeft Amount of nuisance gas unused by the message.
     * @return _ovmGasRefund Amount of gas refunded during the message.
     * @return _data Additional user-provided revert data.
     */
    function _decodeRevertData(
        bytes memory _revertdata
    )
        internal
        pure
        returns (
            RevertFlag _flag,
            uint256 _nuisanceGasLeft,
            uint256 _ovmGasRefund,
            bytes memory _data
        )
    {
        // A length of zero means the call ran out of gas, just return empty data.
        if (_revertdata.length == 0) {
            return (
                RevertFlag.OUT_OF_GAS,
                0,
                0,
                bytes('')
            );
        }

        // ABI decode the incoming data.
        return abi.decode(_revertdata, (RevertFlag, uint256, uint256, bytes));
    }

    /**
     * Causes a message to revert or abort.
     * @param _flag Flag to revert with.
     * @param _data Additional user-provided data.
     */
    function _revertWithFlag(
        RevertFlag _flag,
        bytes memory _data
    )
        internal
        view
    {
        bytes memory revertdata = _encodeRevertData(
            _flag,
            _data
        );

        assembly {
            revert(add(revertdata, 0x20), mload(revertdata))
        }
    }

    /**
     * Causes a message to revert or abort.
     * @param _flag Flag to revert with.
     */
    function _revertWithFlag(
        RevertFlag _flag
    )
        internal
    {
        _revertWithFlag(_flag, bytes(''));
    }


    /******************************************
     * Internal Functions: Nuisance Gas Logic *
     ******************************************/

    /**
     * Computes the nuisance gas limit from the gas limit.
     * @dev This function is currently using a naive implementation whereby the nuisance gas limit
     *      is set to exactly equal the lesser of the gas limit or remaining gas. It's likely that
     *      this implementation is perfectly fine, but we may change this formula later.
     * @param _gasLimit Gas limit to compute from.
     * @return _nuisanceGasLimit Computed nuisance gas limit.
     */
    function _getNuisanceGasLimit(
        uint256 _gasLimit
    )
        internal
        view
        returns (
            uint256 _nuisanceGasLimit
        )
    {
        return _gasLimit < gasleft() ? _gasLimit : gasleft();
    }

    /**
     * Uses a certain amount of nuisance gas.
     * @param _amount Amount of nuisance gas to use.
     */
    function _useNuisanceGas(
        uint256 _amount
    )
        internal
    {
        // Essentially the same as a standard OUT_OF_GAS, except we also retain a record of the gas
        // refund to be given at the end of the transaction.
        if (messageRecord.nuisanceGasLeft < _amount) {
            _revertWithFlag(RevertFlag.EXCEEDS_NUISANCE_GAS);
        }

        messageRecord.nuisanceGasLeft -= _amount;
    }


    /************************************
     * Internal Functions: Gas Metering *
     ************************************/

    /**
     * Checks whether a transaction needs to start a new epoch and does so if necessary.
     * @param _timestamp Transaction timestamp.
     */
    function _checkNeedsNewEpoch(
        uint256 _timestamp
    )
        internal
    {
        if (
            _timestamp >= (
                _getGasMetadata(GasMetadataKey.CURRENT_EPOCH_START_TIMESTAMP)
                + gasMeterConfig.secondsPerEpoch
            )
        ) {
            _putGasMetadata(
                GasMetadataKey.CURRENT_EPOCH_START_TIMESTAMP,
                _timestamp
            );

            _putGasMetadata(
                GasMetadataKey.PREV_EPOCH_SEQUENCER_QUEUE_GAS,
                _getGasMetadata(
                    GasMetadataKey.CUMULATIVE_SEQUENCER_QUEUE_GAS
                )
            );

            _putGasMetadata(
                GasMetadataKey.PREV_EPOCH_L1TOL2_QUEUE_GAS,
                _getGasMetadata(
                    GasMetadataKey.CUMULATIVE_L1TOL2_QUEUE_GAS
                )
            );
        }
    }

    /**
     * Validates the input values of a transaction.
     * @return _valid Whether or not the transaction data is valid.
     */
    function _isValidInput(
        Lib_OVMCodec.Transaction memory _transaction
    )
        view
        internal
        returns (
            bool
        )
    {
        // Prevent reentrancy to run():
        // This check prevents calling run with the default ovmNumber.
        // Combined with the first check in run():
        //      if (transactionContext.ovmNUMBER != DEFAULT_UINT256) { return; }
        // It should be impossible to re-enter since run() returns before any other call frames are created.
        // Since this value is already being written to storage, we save much gas compared to
        // using the standard nonReentrant pattern.
        if (_transaction.blockNumber == DEFAULT_UINT256)  {
            return false;
        }

        if (_isValidGasLimit(_transaction.gasLimit, _transaction.l1QueueOrigin) == false) {
            return false;
        }

        return true;
    }

    /**
     * Validates the gas limit for a given transaction.
     * @param _gasLimit Gas limit provided by the transaction.
     * param _queueOrigin Queue from which the transaction originated.
     * @return _valid Whether or not the gas limit is valid.
     */
    function _isValidGasLimit(
        uint256 _gasLimit,
        Lib_OVMCodec.QueueOrigin // _queueOrigin
    )
        view
        internal
        returns (
            bool _valid
        )
    {
        // Always have to be below the maximum gas limit.
        if (_gasLimit > gasMeterConfig.maxTransactionGasLimit) {
            return false;
        }

        // Always have to be above the minimum gas limit.
        if (_gasLimit < gasMeterConfig.minTransactionGasLimit) {
            return false;
        }

        // TEMPORARY: Gas metering is disabled for minnet.
        return true;
        // GasMetadataKey cumulativeGasKey;
        // GasMetadataKey prevEpochGasKey;
        // if (_queueOrigin == Lib_OVMCodec.QueueOrigin.SEQUENCER_QUEUE) {
        //     cumulativeGasKey = GasMetadataKey.CUMULATIVE_SEQUENCER_QUEUE_GAS;
        //     prevEpochGasKey = GasMetadataKey.PREV_EPOCH_SEQUENCER_QUEUE_GAS;
        // } else {
        //     cumulativeGasKey = GasMetadataKey.CUMULATIVE_L1TOL2_QUEUE_GAS;
        //     prevEpochGasKey = GasMetadataKey.PREV_EPOCH_L1TOL2_QUEUE_GAS;
        // }

        // return (
        //     (
        //         _getGasMetadata(cumulativeGasKey)
        //         - _getGasMetadata(prevEpochGasKey)
        //         + _gasLimit
        //     ) < gasMeterConfig.maxGasPerQueuePerEpoch
        // );
    }

    /**
     * Updates the cumulative gas after a transaction.
     * @param _gasUsed Gas used by the transaction.
     * @param _queueOrigin Queue from which the transaction originated.
     */
    function _updateCumulativeGas(
        uint256 _gasUsed,
        Lib_OVMCodec.QueueOrigin _queueOrigin
    )
        internal
    {
        GasMetadataKey cumulativeGasKey;
        if (_queueOrigin == Lib_OVMCodec.QueueOrigin.SEQUENCER_QUEUE) {
            cumulativeGasKey = GasMetadataKey.CUMULATIVE_SEQUENCER_QUEUE_GAS;
        } else {
            cumulativeGasKey = GasMetadataKey.CUMULATIVE_L1TOL2_QUEUE_GAS;
        }

        _putGasMetadata(
            cumulativeGasKey,
            (
                _getGasMetadata(cumulativeGasKey)
                + gasMeterConfig.minTransactionGasLimit
                + _gasUsed
                - transactionRecord.ovmGasRefund
            )
        );
    }

    /**
     * Retrieves the value of a gas metadata key.
     * @param _key Gas metadata key to retrieve.
     * @return _value Value stored at the given key.
     */
    function _getGasMetadata(
        GasMetadataKey _key
    )
        internal
        returns (
            uint256 _value
        )
    {
        return uint256(_getContractStorage(
            GAS_METADATA_ADDRESS,
            bytes32(uint256(_key))
        ));
    }

    /**
     * Sets the value of a gas metadata key.
     * @param _key Gas metadata key to set.
     * @param _value Value to store at the given key.
     */
    function _putGasMetadata(
        GasMetadataKey _key,
        uint256 _value
    )
        internal
    {
        _putContractStorage(
            GAS_METADATA_ADDRESS,
            bytes32(uint256(_key)),
            bytes32(uint256(_value))
        );
    }


    /*****************************************
     * Internal Functions: Execution Context *
     *****************************************/

    /**
     * Swaps over to a new message context.
     * @param _prevMessageContext Context we're switching from.
     * @param _nextMessageContext Context we're switching to.
     */
    function _switchMessageContext(
        MessageContext memory _prevMessageContext,
        MessageContext memory _nextMessageContext
    )
        internal
    {
        // These conditionals allow us to avoid unneccessary SSTOREs.  However, they do mean that the current storage
        // value for the messageContext MUST equal the _prevMessageContext argument, or an SSTORE might be erroneously skipped.
        if (_prevMessageContext.ovmCALLER != _nextMessageContext.ovmCALLER) {
            messageContext.ovmCALLER = _nextMessageContext.ovmCALLER;
        }

        if (_prevMessageContext.ovmADDRESS != _nextMessageContext.ovmADDRESS) {
            messageContext.ovmADDRESS = _nextMessageContext.ovmADDRESS;
        }

        if (_prevMessageContext.isStatic != _nextMessageContext.isStatic) {
            messageContext.isStatic = _nextMessageContext.isStatic;
        }

        if (_prevMessageContext.ovmCALLVALUE != _nextMessageContext.ovmCALLVALUE) {
            messageContext.ovmCALLVALUE = _nextMessageContext.ovmCALLVALUE;
        }
    }

    /**
     * Initializes the execution context.
     * @param _transaction OVM transaction being executed.
     */
    function _initContext(
        Lib_OVMCodec.Transaction memory _transaction
    )
        internal
    {
        transactionContext.ovmTIMESTAMP = _transaction.timestamp;
        transactionContext.ovmNUMBER = _transaction.blockNumber;
        transactionContext.ovmTXGASLIMIT = _transaction.gasLimit;
        transactionContext.ovmL1QUEUEORIGIN = _transaction.l1QueueOrigin;
        transactionContext.ovmL1TXORIGIN = _transaction.l1TxOrigin;
        transactionContext.ovmGASLIMIT = gasMeterConfig.maxGasPerQueuePerEpoch;

        messageRecord.nuisanceGasLeft = _getNuisanceGasLimit(_transaction.gasLimit);
    }

    /**
     * Resets the transaction and message context.
     */
    function _resetContext()
        internal
    {
        transactionContext.ovmL1TXORIGIN = DEFAULT_ADDRESS;
        transactionContext.ovmTIMESTAMP = DEFAULT_UINT256;
        transactionContext.ovmNUMBER = DEFAULT_UINT256;
        transactionContext.ovmGASLIMIT = DEFAULT_UINT256;
        transactionContext.ovmTXGASLIMIT = DEFAULT_UINT256;
        transactionContext.ovmL1QUEUEORIGIN = Lib_OVMCodec.QueueOrigin.SEQUENCER_QUEUE;

        transactionRecord.ovmGasRefund = DEFAULT_UINT256;

        messageContext.ovmCALLER = DEFAULT_ADDRESS;
        messageContext.ovmADDRESS = DEFAULT_ADDRESS;
        messageContext.isStatic = false;

        messageRecord.nuisanceGasLeft = DEFAULT_UINT256;

        // Reset the ovmStateManager.
        ovmStateManager = iOVM_StateManager(address(0));
    }


    /******************************************
     * Internal Functions: Message Typechecks *
     ******************************************/

    /**
     * Returns whether or not the given message type is a CREATE-type.
     * @param _messageType the message type in question.
     */
    function _isCreateType(
        MessageType _messageType
    )
        internal
        pure
        returns(
            bool
        )
    {
        return (
            _messageType == MessageType.ovmCREATE
            || _messageType == MessageType.ovmCREATE2
        );
    }

    /**
     * Returns whether or not the given message type (potentially) requires the transfer of ETH value along with the message.
     * @param _messageType the message type in question.
     */
    function _isValueType(
        MessageType _messageType
    )
        internal
        pure
        returns(
            bool
        )
    {
        // ovmSTATICCALL and ovmDELEGATECALL types do not accept or transfer value.
        return (
            _messageType == MessageType.ovmCALL
            || _messageType == MessageType.ovmCREATE
            || _messageType == MessageType.ovmCREATE2
        );
    }


    /*****************************
     * L2-only Helper Functions *
     *****************************/

    /**
     * Unreachable helper function for simulating eth_calls with an OVM message context.
     * This function will throw an exception in all cases other than when used as a custom entrypoint in L2 Geth to simulate eth_call.
     * @param _transaction the message transaction to simulate.
     * @param _from the OVM account the simulated call should be from.
     * @param _value the amount of ETH value to send.
     * @param _ovmStateManager the address of the OVM_StateManager precompile in the L2 state.
     */
    function simulateMessage(
        Lib_OVMCodec.Transaction memory _transaction,
        address _from,
        uint256 _value,
        iOVM_StateManager _ovmStateManager
    )
        external
        returns (
            bytes memory
        )
    {
        // Prevent this call from having any effect unless in a custom-set VM frame
        require(msg.sender == address(0));

        // Initialize the EM's internal state, ignoring nuisance gas.
        ovmStateManager = _ovmStateManager;
        _initContext(_transaction);
        messageRecord.nuisanceGasLeft = uint(-1);

        // Set the ovmADDRESS to the _from so that the subsequent call frame "comes from" them.
        messageContext.ovmADDRESS = _from;

        // Execute the desired message.
        bool isCreate = _transaction.entrypoint == address(0);
        if (isCreate) {
            (address created, bytes memory revertData) = ovmCREATE(_transaction.data);
            if (created == address(0)) {
                return abi.encode(false, revertData);
            } else {
                // The eth_call RPC endpoint for to = undefined will return the deployed bytecode
                // in the success case, differing from standard create messages.
                return abi.encode(true, Lib_EthUtils.getCode(created));
            }
        } else {
            (bool success, bytes memory returndata) = ovmCALL(
                _transaction.gasLimit,
                _transaction.entrypoint,
                _value,
                _transaction.data
            );
            return abi.encode(success, returndata);
        }
    }
}
