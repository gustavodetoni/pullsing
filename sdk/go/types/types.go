package types

import (
	pullsingv1 "github.com/gustavodetoni/pullsing/proto/gen/go/pullsing/v1"
)

type BoolFlag struct {
	Key     string
	Enabled bool
	Value   bool
}

type Snapshot struct {
	Revision uint64
	Flags    map[string]BoolFlag
}

type MutationType int

const (
	MutationTypeUnspecified MutationType = iota
	MutationTypeUpsert
	MutationTypeDelete
)

type Mutation struct {
	Type MutationType
	Key  string
	Flag BoolFlag
}

func SnapshotFromProto(in *pullsingv1.Snapshot) Snapshot {
	if in == nil {
		return Snapshot{Flags: map[string]BoolFlag{}}
	}

	flags := make(map[string]BoolFlag, len(in.GetFlags()))
	for _, flag := range in.GetFlags() {
		if flag == nil {
			continue
		}

		boolFlag := BoolFlagFromProto(flag)
		if boolFlag.Key == "" {
			continue
		}

		flags[boolFlag.Key] = boolFlag
	}

	return Snapshot{
		Revision: in.GetRevision(),
		Flags:    flags,
	}
}

func MutationsFromProto(in []*pullsingv1.FlagMutation) []Mutation {
	if len(in) == 0 {
		return nil
	}

	out := make([]Mutation, 0, len(in))
	for _, mutation := range in {
		if mutation == nil {
			continue
		}

		key := mutation.GetKey()
		flag := BoolFlagFromProto(mutation.GetFlag())
		if key == "" {
			key = flag.Key
		}
		if key == "" {
			continue
		}

		out = append(out, Mutation{
			Type: MutationTypeFromProto(mutation.GetType()),
			Key:  key,
			Flag: flag,
		})
	}

	return out
}

func BoolFlagFromProto(in *pullsingv1.Flag) BoolFlag {
	if in == nil {
		return BoolFlag{}
	}

	return BoolFlag{
		Key:     in.GetKey(),
		Enabled: in.GetEnabled(),
		Value:   in.GetBoolValue(),
	}
}

func MutationTypeFromProto(in pullsingv1.MutationType) MutationType {
	switch in {
	case pullsingv1.MutationType_MUTATION_TYPE_UPSERT:
		return MutationTypeUpsert
	case pullsingv1.MutationType_MUTATION_TYPE_DELETE:
		return MutationTypeDelete
	default:
		return MutationTypeUnspecified
	}
}
