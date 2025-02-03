package symbols_table

import (
	"github.com/pherrymason/c3-lsp/pkg/option"
	"github.com/pherrymason/c3-lsp/pkg/symbols"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// SymbolsTable
// Deprecated
type SymbolsTable struct {
	parsedModulesByDocument map[protocol.DocumentUri]UnitModules
	pendingToResolve        PendingToResolve
}

// Deprecated
func NewSymbolsTable() SymbolsTable {
	return SymbolsTable{
		parsedModulesByDocument: make(map[protocol.DocumentUri]UnitModules),
		pendingToResolve:        NewPendingToResolve(),
	}
}

// Deprecated
func (st *SymbolsTable) Register(unitModules UnitModules, pendingToResolve PendingToResolve) {
	st.parsedModulesByDocument[unitModules.DocId()] = unitModules

	// Merge pendingToResolve types
	for moduleName, types := range pendingToResolve.typesByModule {
		st.pendingToResolve.typesByModule[moduleName] = append(
			st.pendingToResolve.typesByModule[moduleName],
			types...,
		)
	}

	st.pendingToResolve.subtyptingToResolve = append(
		st.pendingToResolve.subtyptingToResolve,
		pendingToResolve.subtyptingToResolve...,
	)

	st.resolveTypes()
}

func (st *SymbolsTable) DeleteDocument(docId string) {
	delete(st.parsedModulesByDocument, docId)
}
func (st *SymbolsTable) RenameDocument(oldDocId string, newDocId string) {
	if val, ok := st.parsedModulesByDocument[oldDocId]; ok {
		// Asignar el valor a la nueva clave
		st.parsedModulesByDocument[newDocId] = val
		// Eliminar la clave antigua
		delete(st.parsedModulesByDocument, oldDocId)
	}
}

func (st *SymbolsTable) GetByDoc(docId string) *UnitModules {
	value, _ := st.parsedModulesByDocument[docId]
	return &value
}

func (st SymbolsTable) All() map[protocol.DocumentUri]UnitModules {
	return st.parsedModulesByDocument
}

// Processes pending Types to resolve, searching
func (st *SymbolsTable) resolveTypes() {

	// Review all pending types, and see if we can resolve them
	for moduleName, typesContext := range st.pendingToResolve.typesByModule {
		// Reviewing types in `moduleName`
		for x := len(typesContext) - 1; x >= 0; x-- {
			if typesContext[x].IsSolved() {
				continue
			}

			st.tryToSolveType(&typesContext[x], moduleName)
		}
	}

	// Resolve inline sub struct
	st.expendStructSubtypes()
}

func (st *SymbolsTable) tryToSolveType(typeContext *PendingTypeContext, moduleName string) {
	if len(typeContext.contextModule.Imports) > 0 {
		// Check inside imported modules
		for _, imported := range typeContext.contextModule.Imports {
			mpath := symbols.NewModulePathFromString(imported)

			// Loop through project modules searching for `imported`
			for _, parsedModules := range st.parsedModulesByDocument {

				if !parsedModules.HasExplicitlyImportedModules(mpath) {
					continue
				}

				moduleOption := st.findTypeInModules(typeContext.vType)
				if moduleOption.IsSome() {
					// Found in same file! Fix it
					typeContext.vType.SetModule(moduleOption.Get())
					typeContext.Solve()
				}

			}
		}
		// Not found! Keep it registered as pending
	} else {
		moduleOption := st.findTypeInModules(typeContext.vType)
		if moduleOption.IsSome() {
			// Found in same file! Fix it
			typeContext.vType.SetModule(moduleOption.Get())
			typeContext.Solve()
		}
	}
}

func (st *SymbolsTable) findTypeInModules(vType *symbols.Type) option.Option[string] {
	vTypeName := vType.GetName()
	for _, parsedModules := range st.parsedModulesByDocument {
		// Use ModuleIds() to cheaply copy the slice of module names
		// Using Modules() appears to lead to expensive clones
		for _, moduleName := range parsedModules.ModuleIds() {
			module := parsedModules.Get(moduleName)

			// Use ChildrenNames() since we are only comparing names
			// Avoid expensive cloning of children
			for _, childName := range module.ChildrenNames() {
				if childName == vTypeName {
					// Found!
					return option.Some(module.GetName())
				}
			}
		}
	}

	return option.None[string]()
}

// Resolves inline sub structs
func (st *SymbolsTable) expendStructSubtypes() {

	if len(st.pendingToResolve.subtyptingToResolve) == 0 {
		return
	}

	for _, struktWithSubtyping := range st.pendingToResolve.subtyptingToResolve {
		for _, inlinedMemberName := range struktWithSubtyping.members {

			// Go through all parsed modules searching structs with members to inline
			for _, parsedModules := range st.parsedModulesByDocument {
				for _, module := range parsedModules.Modules() {
					// Search
					for _, strukt := range module.Structs {
						if strukt.GetName() == inlinedMemberName.GetName() {
							struktWithSubtyping.strukt.InheritMembersFrom(inlinedMemberName.GetName(), strukt)
						}
					}
				}
			}
		}
	}

	st.pendingToResolve.subtyptingToResolve = []StructWithSubtyping{}
}
