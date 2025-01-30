// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Script } from "forge-std/Script.sol";
import { console2 as console } from "forge-std/console2.sol";
import { stdJson } from "forge-std/StdJson.sol";

import { Drippie } from "src/periphery/drippie/Drippie.sol";
import { CheckBalanceLow } from "src/periphery/drippie/dripchecks/CheckBalanceLow.sol";
import { CheckSecrets } from "src/periphery/drippie/dripchecks/CheckSecrets.sol";

/// @title DrippieConfig
/// @notice Loads Drippie configuration from a JSON file.
contract DrippieConfig is Script {
    /// @notice Error emitted when an unknown drip check is encountered.
    error UnknownDripCheck(string name);

    /// @notice Struct describing a dripcheck.
    struct DripCheck {
        string name;
        address addr;
    }

    /// @notice Drip configuration with only name and dripcheck.
    struct CoreDripConfig {
        string name;
        string dripcheck;
    }

    /// @notice Full drip configuration.
    struct FullDripConfig {
        string name;
        string dripcheck;
        bytes checkparams;
        address recipient;
        uint256 value;
        uint256 interval;
        bytes data;
    }

    /// @notice JSON configuration file represented as string.
    string internal _json;

    /// @notice Drippie contract.
    Drippie public drippie;

    /// @notice Prefix for the configuration file.
    string public prefix;

    /// @notice Drip configuration array.
    FullDripConfig[] public drips;

    /// @notice Mapping of drip names in the config.
    mapping(string => bool) public names;

    /// @notice Mapping of dripcheck names to addresses.
    mapping(string => address) public dripchecks;

    /// @param _path Path to the configuration file.
    constructor(string memory _path) {
        // Load the configuration file.
        console.log("DrippieConfig: reading file %s", _path);
        try vm.readFile(_path) returns (string memory data_) {
            _json = data_;
        } catch {
            console.log("WARNING: unable to read config, do not deploy unless you are not using config");
            return;
        }

        // Load the Drippie contract address.
        drippie = Drippie(payable(stdJson.readAddress(_json, "$.drippie")));

        // Load the prefix.
        prefix = stdJson.readString(_json, "$.prefix");

        // Determine the number of drips.
        // In an ideal world we'd be able to load this array in one go by parsing it as an array
        // of structs that include the checkparams as bytes. Unfortunately, Foundry parses the
        // checkparams as a tuple which can't be parsed in a generic way (since Solidity does not
        // support generics). As a result, we first parse the array as a simplified struct that
        // only includes the first two fields so that we can determine the number of drips. We then
        // iterate over the array and parse the full struct for each drip somewhat manually.
        CoreDripConfig[] memory corecfg = abi.decode(stdJson.parseRaw(_json, "$.drips"), (CoreDripConfig[]));
        console.log("DrippieConfig: found %d drips", corecfg.length);

        // Load the dripchecks.
        DripCheck[] memory checks = abi.decode(stdJson.parseRaw(_json, "$.dripchecks"), (DripCheck[]));
        for (uint256 i = 0; i < checks.length; i++) {
            dripchecks[checks[i].name] = checks[i].addr;
        }

        // Iterate and parse all of the drips.
        for (uint256 i = 0; i < corecfg.length; i++) {
            // Log so we know what's being loaded.
            string memory name = corecfg[i].name;
            string memory fullname = string.concat(prefix, "_", name);
            console.log("DrippieConfig: attempting to load config for %s", fullname);

            // Make sure the dripcheck is deployed.
            string memory dripcheck = corecfg[i].dripcheck;
            console.log("DrippieConfig: attempting to get address for %s", dripcheck);
            mustGetDripCheck(dripcheck);

            // Generate the base JSON path string.
            string memory p = string.concat("$.drips[", vm.toString(i), "]");

            // Load the checkparams as bytes.
            bytes memory checkparams = stdJson.parseRaw(_json, string.concat(p, ".02__checkparams"));

            // Determine if the parameters are decodable.
            console.log("DrippieConfig: attempting to decode check parameters for %s", fullname);
            if (strcmp(dripcheck, "CheckBalanceLow")) {
                abi.decode(checkparams, (CheckBalanceLow.Params));
            } else if (strcmp(dripcheck, "CheckSecrets")) {
                abi.decode(checkparams, (CheckSecrets.Params));
            } else if (strcmp(dripcheck, "CheckTrue")) {
                // No parameters to decode.
            } else {
                console.log("ERROR: unknown drip configuration %s", dripcheck);
                revert UnknownDripCheck(dripcheck);
            }

            // Parse all the easy stuff first.
            console.log("DrippieConfig: attempting to load core configuration for %s", name);
            FullDripConfig memory dripcfg = FullDripConfig({
                name: fullname,
                dripcheck: dripcheck,
                checkparams: checkparams,
                recipient: stdJson.readAddress(_json, string.concat(p, ".03__recipient")),
                value: stdJson.readUint(_json, string.concat(p, ".04__value")),
                interval: stdJson.readUint(_json, string.concat(p, ".05__interval")),
                data: stdJson.parseRaw(_json, string.concat(p, ".06__data"))
            });

            // Ok we're good to go.
            drips.push(dripcfg);
            names[fullname] = true;
        }
    }

    /// @notice Returns the number of drips in the configuration.
    function dripsLength() public view returns (uint256) {
        return drips.length;
    }

    /// @notice Returns the drip configuration at the given index as ABI-encoded bytes.
    function drip(uint256 _index) public view returns (bytes memory) {
        return abi.encode(drips[_index]);
    }

    /// @notice Retrieves the address of a dripcheck and reverts if it is not found.
    /// @param _name Name of the dripcheck.
    /// @return addr_ Address of the dripcheck.
    function mustGetDripCheck(string memory _name) public view returns (address addr_) {
        addr_ = dripchecks[_name];
        require(addr_ != address(0), "DrippieConfig: unknown dripcheck");
    }

    /// @notice Check if two strings are equal.
    /// @param _a First string.
    /// @param _b Second string.
    /// @return True if the strings are equal, false otherwise.
    function strcmp(string memory _a, string memory _b) internal pure returns (bool) {
        return keccak256(bytes(_a)) == keccak256(bytes(_b));
    }
}
