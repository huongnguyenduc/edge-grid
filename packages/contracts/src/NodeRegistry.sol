// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

contract NodeRegistry is Ownable {
    struct NodeInfo {
        string uri;
        string[] models;
        uint256 pricePerUnit;
        bool active;
        uint256 registeredAt;
    }

    mapping(address => NodeInfo) public nodes;
    address[] public nodeList;

    event NodeRegistered(address indexed node, string uri, string[] models, uint256 pricePerUnit);
    event NodeUpdated(address indexed node, string uri, string[] models, uint256 pricePerUnit);
    event NodeDeactivated(address indexed node);

    error NodeNotRegistered();
    error InvalidPrice();
    error EmptyURI();

    constructor() Ownable(msg.sender) {}

    function register(
        string calldata uri,
        string[] calldata models,
        uint256 pricePerUnit
    ) external {
        if (bytes(uri).length == 0) revert EmptyURI();
        if (pricePerUnit == 0) revert InvalidPrice();

        // Copy models array to storage
        string[] storage nodeModels = nodes[msg.sender].models;
        // Clear existing models
        while (nodeModels.length > 0) {
            nodeModels.pop();
        }
        // Add new models
        for (uint256 i = 0; i < models.length; i++) {
            nodeModels.push(models[i]);
        }

        nodes[msg.sender] = NodeInfo({
            uri: uri,
            models: nodeModels,
            pricePerUnit: pricePerUnit,
            active: true,
            registeredAt: block.timestamp
        });

        // Add to list if not already registered
        if (bytes(nodes[msg.sender].uri).length == 0) {
            nodeList.push(msg.sender);
        }

        emit NodeRegistered(msg.sender, uri, models, pricePerUnit);
    }

    function update(
        string calldata uri,
        string[] calldata models,
        uint256 pricePerUnit
    ) external {
        if (!nodes[msg.sender].active) revert NodeNotRegistered();
        if (bytes(uri).length == 0) revert EmptyURI();
        if (pricePerUnit == 0) revert InvalidPrice();

        // Copy models array to storage
        string[] storage nodeModels = nodes[msg.sender].models;
        // Clear existing models
        while (nodeModels.length > 0) {
            nodeModels.pop();
        }
        // Add new models
        for (uint256 i = 0; i < models.length; i++) {
            nodeModels.push(models[i]);
        }

        nodes[msg.sender].uri = uri;
        nodes[msg.sender].pricePerUnit = pricePerUnit;

        emit NodeUpdated(msg.sender, uri, models, pricePerUnit);
    }

    function deactivate() external {
        if (!nodes[msg.sender].active) revert NodeNotRegistered();
        
        nodes[msg.sender].active = false;
        emit NodeDeactivated(msg.sender);
    }

    function getNodeInfo(address node) external view returns (NodeInfo memory) {
        return nodes[node];
    }

    function isRegistered(address node) external view returns (bool) {
        return nodes[node].active;
    }

    function getNodeCount() external view returns (uint256) {
        return nodeList.length;
    }

    function getNodeAt(uint256 index) external view returns (address) {
        require(index < nodeList.length, "Index out of bounds");
        return nodeList[index];
    }
}
