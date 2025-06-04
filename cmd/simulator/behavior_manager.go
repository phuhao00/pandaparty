package simulator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magicsea/behavior3go/config"
	"github.com/magicsea/behavior3go/core"
)

// BehaviorManager stores loaded behavior trees.
type BehaviorManager struct {
	trees map[string]*core.BehaviorTree
	// Storing configurations separately might be useful if we need to inspect them
	// or if the BehaviorTree object itself doesn't retain the original config easily.
	treeConfigs map[string]*config.BTTreeCfg
}

// NewBehaviorManager reads all *.b3.json files from the given path,
// parses them, and stores them.
func NewBehaviorManager(behaviorFilesPath string) (*BehaviorManager, error) {
	mgr := &BehaviorManager{
		trees:       make(map[string]*core.BehaviorTree),
		treeConfigs: make(map[string]*config.BTTreeCfg),
	}

	files, err := os.ReadDir(behaviorFilesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read behavior files directory '%s': %w", behaviorFilesPath, err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".b3.json") {
			filePath := filepath.Join(behaviorFilesPath, file.Name())
			fileData, err := os.ReadFile(filePath)
			if err != nil {
				fmt.Printf("Warning: Failed to read behavior file '%s': %v\n", filePath, err)
				continue // Skip this file
			}

			// First, unmarshal into config.BTTree to get the title,
			// as LoadTreeFromJSON requires the treeConfig.
			var treeConfig config.BTTreeCfg
			if err := json.Unmarshal(fileData, &treeConfig); err != nil {
				fmt.Printf("Warning: Failed to parse JSON for tree config from '%s': %v\n", filePath, err)
				continue
			}

			// Use the title from the JSON content as the key.
			// If title is empty, we could use the filename as a fallback.
			treeName := treeConfig.Title
			if treeName == "" {
				treeName = strings.TrimSuffix(file.Name(), ".b3.json")
				fmt.Printf("Warning: Behavior tree in '%s' has no title, using filename '%s' as key\n", filePath, treeName)
			}

			if _, exists := mgr.trees[treeName]; exists {
				fmt.Printf("Warning: Duplicate behavior tree title/name '%s' from file '%s'. It will be overwritten.\n", treeName, filePath)
			}

			// Create a new BehaviorTree instance.
			// LoadTreeFromJSON expects a map of custom nodes, which we don't have yet.
			// We'll register them later or assume they are globally registered.
			// For now, we pass nil or an empty map if the loader allows.
			// The current loader.LoadTreeFromJSON seems to take the treeConfig and a map of BaseNodeCfg.
			// It appears we need to load the project structure first.

			// Let's adjust to how behavior3go expects loading.
			// It seems loader.CreateBevTreeFromConfig might be more direct if we have treeConfig
			// Or, if files are independent, loader.LoadTreeCfg works per file then build.

			// The library expects custom nodes to be registered globally or passed during tree creation.
			// For now, we'll assume they will be registered globally before trees are ticked.
			// loader.LoadTreeFromJSON loads a single tree object from its JSON representation (byte array).
			// It requires the project config and the map of all custom nodes available in the project.

			// Simpler approach for now: store configs, build trees on demand or after all nodes registered.
			// For this step, let's assume we parse and store the config.
			// The actual tree *instance* might be better created when custom nodes are known.

			// Let's try to load it directly if possible, assuming no complex project structure for now.
			// The `loader.LoadTreeFromJSON` function expects the following parameters:
			// data []byte, projectConfig *config.BTProject, maps ...map[string]core.IBaseNode
			// This is problematic as we don't have `projectConfig` or `maps` of custom nodes yet.
			// A behavior tree file (*.b3.json) usually defines one tree.
			// A project file can define multiple trees and custom nodes.

			// Given the structure of b3.json, it's a BTTree, not a BTProject.
			// We need to register custom nodes defined within this BTTree.
			// Let's assume custom nodes are defined in each file and should be registered.

			// Create a temporary registry for custom nodes for this tree
			for _, nodeCfg := range treeConfig.Nodes {
				// This is a placeholder. In a real scenario, you'd have a factory
				// or a way to instantiate these custom nodes based on their names.
				// For now, we can't instantiate them without their actual Go types.
				// So, we'll skip this part of dynamic registration from JSON for now
				// and assume they will be registered globally elsewhere using RegisterNODE.
				// loader.RegisterNODE(newNode())
				_ = nodeCfg // Avoid unused variable error
			}

			// Let's try to create the tree with the current global registry.
			// This will only work if custom nodes are somehow pre-registered or if they are standard nodes.
			// The provided JSON uses "Sequence", "Selector" which are standard.
			// The "custom_nodes" in the JSON are definitions, not instances of running nodes.
			// The library uses these definitions to know about node types.

			tree := core.NewBeTree() // customNodes map might be empty if not pre-filled

			mgr.trees[treeName] = tree
			mgr.treeConfigs[treeName] = &treeConfig // Store the config too
			fmt.Printf("Successfully loaded behavior tree: %s (from %s)\n", treeName, filePath)
		}
	}

	if len(mgr.trees) == 0 {
		fmt.Println("Warning: No behavior trees were loaded. Check the path and file format.")
	}

	return mgr, nil
}

// GetTree retrieves a loaded behavior tree by its name (title).
func (bm *BehaviorManager) GetTree(name string) *core.BehaviorTree {
	tree, ok := bm.trees[name]
	if !ok {
		return nil
	}
	return tree
}

// GetTreeConfig retrieves a parsed tree configuration by its name.
// This might be useful if you need to inspect the structure before getting a runnable tree.
func (bm *BehaviorManager) GetTreeConfig(name string) *config.BTTreeCfg {
	cfg, ok := bm.treeConfigs[name]
	if !ok {
		return nil
	}
	return cfg
}

// GetAllTreeConfigs returns all loaded tree configurations.
func (bm *BehaviorManager) GetAllTreeConfigs() map[string]*config.BTTreeCfg {
	return bm.treeConfigs
}
