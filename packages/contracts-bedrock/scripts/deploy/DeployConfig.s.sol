// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";
import { console2 as console } from "forge-std/console2.sol";
import { stdJson } from "forge-std/StdJson.sol";
import { Process } from "scripts/libraries/Process.sol";
import { Config, Fork, ForkUtils } from "scripts/libraries/Config.sol";

/// @title DeployConfig
/// @notice Represents the configuration required to deploy the system. It is expected
///         to read the file from JSON. A future improvement would be to have fallback
///         values if they are not defined in the JSON themselves.
contract DeployConfig is Script {
    using stdJson for string;
    using ForkUtils for Fork;

    /// @notice Represents an unset offset value, as opposed to 0, which denotes no-offset.
    uint256 constant NULL_OFFSET = type(uint256).max;

    string internal _json;

    address public finalSystemOwner;
    address public superchainConfigGuardian;
    uint256 public l1ChainID;
    uint256 public l2ChainID;
    uint256 public l2BlockTime;
    uint256 public l2GenesisDeltaTimeOffset;
    uint256 public l2GenesisEcotoneTimeOffset;
    uint256 public l2GenesisFjordTimeOffset;
    uint256 public l2GenesisGraniteTimeOffset;
    uint256 public l2GenesisHoloceneTimeOffset;
    uint256 public maxSequencerDrift;
    uint256 public sequencerWindowSize;
    uint256 public channelTimeout;
    address public p2pSequencerAddress;
    address public batchInboxAddress;
    address public batchSenderAddress;
    uint256 public l2OutputOracleSubmissionInterval;
    int256 internal _l2OutputOracleStartingTimestamp;
    uint256 public l2OutputOracleStartingBlockNumber;
    address public l2OutputOracleProposer;
    address public l2OutputOracleChallenger;
    uint256 public finalizationPeriodSeconds;
    bool public fundDevAccounts;
    address public proxyAdminOwner;
    address public baseFeeVaultRecipient;
    uint256 public baseFeeVaultMinimumWithdrawalAmount;
    uint256 public baseFeeVaultWithdrawalNetwork;
    address public l1FeeVaultRecipient;
    uint256 public l1FeeVaultMinimumWithdrawalAmount;
    uint256 public l1FeeVaultWithdrawalNetwork;
    address public sequencerFeeVaultRecipient;
    uint256 public sequencerFeeVaultMinimumWithdrawalAmount;
    uint256 public sequencerFeeVaultWithdrawalNetwork;
    string public governanceTokenName;
    string public governanceTokenSymbol;
    address public governanceTokenOwner;
    uint256 public l2GenesisBlockGasLimit;
    uint32 public basefeeScalar;
    uint32 public blobbasefeeScalar;
    bool public enableGovernance;
    uint256 public eip1559Denominator;
    uint256 public eip1559Elasticity;
    uint256 public faultGameAbsolutePrestate;
    uint256 public faultGameGenesisBlock;
    bytes32 public faultGameGenesisOutputRoot;
    uint256 public faultGameMaxDepth;
    uint256 public faultGameSplitDepth;
    uint256 public faultGameClockExtension;
    uint256 public faultGameMaxClockDuration;
    uint256 public faultGameWithdrawalDelay;
    uint256 public preimageOracleMinProposalSize;
    uint256 public preimageOracleChallengePeriod;
    uint256 public systemConfigStartBlock;
    uint256 public requiredProtocolVersion;
    uint256 public recommendedProtocolVersion;
    uint256 public proofMaturityDelaySeconds;
    uint256 public disputeGameFinalityDelaySeconds;
    uint256 public respectedGameType;
    bool public useFaultProofs;
    bool public useAltDA;
    string public daCommitmentType;
    uint256 public daChallengeWindow;
    uint256 public daResolveWindow;
    uint256 public daBondSize;
    uint256 public daResolverRefundPercentage;

    bool public useCustomGasToken;
    address public customGasTokenAddress;

    bool public useInterop;

    function read(string memory _path) public {
        console.log("DeployConfig: reading file %s", _path);
        try vm.readFile(_path) returns (string memory data_) {
            _json = data_;
        } catch {
            require(false, string.concat("DeployConfig: cannot find deploy config file at ", _path));
        }

        finalSystemOwner = stdJson.readAddress(_json, "$.finalSystemOwner");
        superchainConfigGuardian = stdJson.readAddress(_json, "$.superchainConfigGuardian");
        l1ChainID = stdJson.readUint(_json, "$.l1ChainID");
        l2ChainID = stdJson.readUint(_json, "$.l2ChainID");
        l2BlockTime = stdJson.readUint(_json, "$.l2BlockTime");

        l2GenesisDeltaTimeOffset = _readOr(_json, "$.l2GenesisDeltaTimeOffset", NULL_OFFSET);
        l2GenesisEcotoneTimeOffset = _readOr(_json, "$.l2GenesisEcotoneTimeOffset", NULL_OFFSET);
        l2GenesisFjordTimeOffset = _readOr(_json, "$.l2GenesisFjordTimeOffset", NULL_OFFSET);
        l2GenesisGraniteTimeOffset = _readOr(_json, "$.l2GenesisGraniteTimeOffset", NULL_OFFSET);
        l2GenesisHoloceneTimeOffset = _readOr(_json, "$.l2GenesisHoloceneTimeOffset", NULL_OFFSET);

        maxSequencerDrift = stdJson.readUint(_json, "$.maxSequencerDrift");
        sequencerWindowSize = stdJson.readUint(_json, "$.sequencerWindowSize");
        channelTimeout = stdJson.readUint(_json, "$.channelTimeout");
        p2pSequencerAddress = stdJson.readAddress(_json, "$.p2pSequencerAddress");
        batchInboxAddress = stdJson.readAddress(_json, "$.batchInboxAddress");
        batchSenderAddress = stdJson.readAddress(_json, "$.batchSenderAddress");
        l2OutputOracleSubmissionInterval = stdJson.readUint(_json, "$.l2OutputOracleSubmissionInterval");
        _l2OutputOracleStartingTimestamp = stdJson.readInt(_json, "$.l2OutputOracleStartingTimestamp");
        l2OutputOracleStartingBlockNumber = stdJson.readUint(_json, "$.l2OutputOracleStartingBlockNumber");
        l2OutputOracleProposer = stdJson.readAddress(_json, "$.l2OutputOracleProposer");
        l2OutputOracleChallenger = stdJson.readAddress(_json, "$.l2OutputOracleChallenger");
        finalizationPeriodSeconds = stdJson.readUint(_json, "$.finalizationPeriodSeconds");
        fundDevAccounts = _readOr(_json, "$.fundDevAccounts", false);
        proxyAdminOwner = stdJson.readAddress(_json, "$.proxyAdminOwner");
        baseFeeVaultRecipient = stdJson.readAddress(_json, "$.baseFeeVaultRecipient");
        baseFeeVaultMinimumWithdrawalAmount = stdJson.readUint(_json, "$.baseFeeVaultMinimumWithdrawalAmount");
        baseFeeVaultWithdrawalNetwork = stdJson.readUint(_json, "$.baseFeeVaultWithdrawalNetwork");
        l1FeeVaultRecipient = stdJson.readAddress(_json, "$.l1FeeVaultRecipient");
        l1FeeVaultMinimumWithdrawalAmount = stdJson.readUint(_json, "$.l1FeeVaultMinimumWithdrawalAmount");
        l1FeeVaultWithdrawalNetwork = stdJson.readUint(_json, "$.l1FeeVaultWithdrawalNetwork");
        sequencerFeeVaultRecipient = stdJson.readAddress(_json, "$.sequencerFeeVaultRecipient");
        sequencerFeeVaultMinimumWithdrawalAmount = stdJson.readUint(_json, "$.sequencerFeeVaultMinimumWithdrawalAmount");
        sequencerFeeVaultWithdrawalNetwork = stdJson.readUint(_json, "$.sequencerFeeVaultWithdrawalNetwork");
        governanceTokenName = stdJson.readString(_json, "$.governanceTokenName");
        governanceTokenSymbol = stdJson.readString(_json, "$.governanceTokenSymbol");
        governanceTokenOwner = stdJson.readAddress(_json, "$.governanceTokenOwner");
        l2GenesisBlockGasLimit = stdJson.readUint(_json, "$.l2GenesisBlockGasLimit");
        basefeeScalar = uint32(_readOr(_json, "$.gasPriceOracleBaseFeeScalar", 1368));
        blobbasefeeScalar = uint32(_readOr(_json, "$.gasPriceOracleBlobBaseFeeScalar", 810949));

        enableGovernance = stdJson.readBool(_json, "$.enableGovernance");
        eip1559Denominator = stdJson.readUint(_json, "$.eip1559Denominator");
        eip1559Elasticity = stdJson.readUint(_json, "$.eip1559Elasticity");
        systemConfigStartBlock = stdJson.readUint(_json, "$.systemConfigStartBlock");
        requiredProtocolVersion = stdJson.readUint(_json, "$.requiredProtocolVersion");
        recommendedProtocolVersion = stdJson.readUint(_json, "$.recommendedProtocolVersion");

        useFaultProofs = _readOr(_json, "$.useFaultProofs", false);
        proofMaturityDelaySeconds = _readOr(_json, "$.proofMaturityDelaySeconds", 0);
        disputeGameFinalityDelaySeconds = _readOr(_json, "$.disputeGameFinalityDelaySeconds", 0);
        respectedGameType = _readOr(_json, "$.respectedGameType", 0);

        faultGameAbsolutePrestate = stdJson.readUint(_json, "$.faultGameAbsolutePrestate");
        faultGameMaxDepth = stdJson.readUint(_json, "$.faultGameMaxDepth");
        faultGameSplitDepth = stdJson.readUint(_json, "$.faultGameSplitDepth");
        faultGameClockExtension = stdJson.readUint(_json, "$.faultGameClockExtension");
        faultGameMaxClockDuration = stdJson.readUint(_json, "$.faultGameMaxClockDuration");
        faultGameGenesisBlock = stdJson.readUint(_json, "$.faultGameGenesisBlock");
        faultGameGenesisOutputRoot = stdJson.readBytes32(_json, "$.faultGameGenesisOutputRoot");
        faultGameWithdrawalDelay = stdJson.readUint(_json, "$.faultGameWithdrawalDelay");

        preimageOracleMinProposalSize = stdJson.readUint(_json, "$.preimageOracleMinProposalSize");
        preimageOracleChallengePeriod = stdJson.readUint(_json, "$.preimageOracleChallengePeriod");

        useAltDA = _readOr(_json, "$.useAltDA", false);
        daCommitmentType = _readOr(_json, "$.daCommitmentType", "KeccakCommitment");
        daChallengeWindow = _readOr(_json, "$.daChallengeWindow", 1000);
        daResolveWindow = _readOr(_json, "$.daResolveWindow", 1000);
        daBondSize = _readOr(_json, "$.daBondSize", 1000000000);
        daResolverRefundPercentage = _readOr(_json, "$.daResolverRefundPercentage", 0);

        useCustomGasToken = _readOr(_json, "$.useCustomGasToken", false);
        customGasTokenAddress = _readOr(_json, "$.customGasTokenAddress", address(0));

        useInterop = _readOr(_json, "$.useInterop", false);
    }

    function fork() public view returns (Fork fork_) {
        // let env var take precedence
        fork_ = Config.fork();
        if (fork_ == Fork.NONE) {
            // Will revert if no deploy config can be found either.
            fork_ = latestGenesisFork();
            console.log("DeployConfig: using deploy config fork: %s", fork_.toString());
        } else {
            console.log("DeployConfig: using env var fork: %s", fork_.toString());
        }
    }

    function l1StartingBlockTag() public returns (bytes32) {
        try vm.parseJsonBytes32(_json, "$.l1StartingBlockTag") returns (bytes32 tag_) {
            return tag_;
        } catch {
            try vm.parseJsonString(_json, "$.l1StartingBlockTag") returns (string memory tag_) {
                return _getBlockByTag(tag_);
            } catch {
                try vm.parseJsonUint(_json, "$.l1StartingBlockTag") returns (uint256 tag_) {
                    return _getBlockByTag(vm.toString(tag_));
                } catch { }
            }
        }
        revert(
            "DeployConfig: l1StartingBlockTag must be a bytes32, string or uint256 or cannot fetch l1StartingBlockTag"
        );
    }

    function l2OutputOracleStartingTimestamp() public returns (uint256) {
        if (_l2OutputOracleStartingTimestamp < 0) {
            bytes32 tag = l1StartingBlockTag();
            string memory cmd = string.concat("cast block ", vm.toString(tag), " --json | jq .timestamp");
            string memory res = Process.bash(cmd);
            return stdJson.readUint(res, "");
        }
        return uint256(_l2OutputOracleStartingTimestamp);
    }

    /// @notice Allow the `useAltDA` config to be overridden in testing environments
    function setUseAltDA(bool _useAltDA) public {
        useAltDA = _useAltDA;
    }

    /// @notice Allow the `useFaultProofs` config to be overridden in testing environments
    function setUseFaultProofs(bool _useFaultProofs) public {
        useFaultProofs = _useFaultProofs;
    }

    /// @notice Allow the `useInterop` config to be overridden in testing environments
    function setUseInterop(bool _useInterop) public {
        useInterop = _useInterop;
    }

    /// @notice Allow the `fundDevAccounts` config to be overridden.
    function setFundDevAccounts(bool _fundDevAccounts) public {
        fundDevAccounts = _fundDevAccounts;
    }

    /// @notice Allow the `useCustomGasToken` config to be overridden in testing environments
    function setUseCustomGasToken(address _token) public {
        useCustomGasToken = true;
        customGasTokenAddress = _token;
    }

    function latestGenesisFork() internal view returns (Fork) {
        if (l2GenesisHoloceneTimeOffset == 0) {
            return Fork.HOLOCENE;
        } else if (l2GenesisGraniteTimeOffset == 0) {
            return Fork.GRANITE;
        } else if (l2GenesisFjordTimeOffset == 0) {
            return Fork.FJORD;
        } else if (l2GenesisEcotoneTimeOffset == 0) {
            return Fork.ECOTONE;
        } else if (l2GenesisDeltaTimeOffset == 0) {
            return Fork.DELTA;
        }
        revert("DeployConfig: no supported fork active at genesis");
    }

    function _getBlockByTag(string memory _tag) internal returns (bytes32) {
        string memory cmd = string.concat("cast block ", _tag, " --json | jq -r .hash");
        bytes memory res = bytes(Process.bash(cmd));
        return abi.decode(res, (bytes32));
    }

    function _readOr(string memory _jsonInp, string memory _key, bool _defaultValue) internal view returns (bool) {
        return _jsonInp.readBoolOr(_key, _defaultValue);
    }

    function _readOr(
        string memory _jsonInp,
        string memory _key,
        uint256 _defaultValue
    )
        internal
        view
        returns (uint256)
    {
        return (vm.keyExistsJson(_jsonInp, _key) && !_isNull(_json, _key)) ? _jsonInp.readUint(_key) : _defaultValue;
    }

    function _readOr(
        string memory _jsonInp,
        string memory _key,
        address _defaultValue
    )
        internal
        view
        returns (address)
    {
        return _jsonInp.readAddressOr(_key, _defaultValue);
    }

    function _isNull(string memory _jsonInp, string memory _key) internal pure returns (bool) {
        string memory value = _jsonInp.readString(_key);
        return (keccak256(bytes(value)) == keccak256(bytes("null")));
    }

    function _readOr(
        string memory _jsonInp,
        string memory _key,
        string memory _defaultValue
    )
        internal
        view
        returns (string memory)
    {
        return _jsonInp.readStringOr(_key, _defaultValue);
    }
}
