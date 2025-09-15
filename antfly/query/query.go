//go:generate go tool oapi-codegen --config=cfg.yaml ../../../bleve-query-openapi.yaml
package query

// FuzzinessInt creates a Fuzziness from an int32. Panics on error.
func FuzzinessInt(v int32) Fuzziness {
	var f Fuzziness
	if err := f.FromFuzziness0(v); err != nil {
		panic(err)
	}
	return f
}

// FuzzinessStr creates a Fuzziness from a string. Panics on error.
func FuzzinessStr(v string) Fuzziness {
	var f Fuzziness
	if err := f.FromFuzziness1(Fuzziness1(v)); err != nil {
		panic(err)
	}
	return f
}

// ToQuery creates a Query from a TermQuery. Panics on error.
func (v TermQuery) ToQuery() Query {
	var q Query
	if err := q.FromTermQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a MatchQuery. Panics on error.
func (v MatchQuery) ToQuery() Query {
	var q Query
	if err := q.FromMatchQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a MatchPhraseQuery. Panics on error.
func (v MatchPhraseQuery) ToQuery() Query {
	var q Query
	if err := q.FromMatchPhraseQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a PhraseQuery. Panics on error.
func (v PhraseQuery) ToQuery() Query {
	var q Query
	if err := q.FromPhraseQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a MultiPhraseQuery. Panics on error.
func (v MultiPhraseQuery) ToQuery() Query {
	var q Query
	if err := q.FromMultiPhraseQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a FuzzyQuery. Panics on error.
func (v FuzzyQuery) ToQuery() Query {
	var q Query
	if err := q.FromFuzzyQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a PrefixQuery. Panics on error.
func (v PrefixQuery) ToQuery() Query {
	var q Query
	if err := q.FromPrefixQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a RegexpQuery. Panics on error.
func (v RegexpQuery) ToQuery() Query {
	var q Query
	if err := q.FromRegexpQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a WildcardQuery. Panics on error.
func (v WildcardQuery) ToQuery() Query {
	var q Query
	if err := q.FromWildcardQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a QueryStringQuery. Panics on error.
func (v QueryStringQuery) ToQuery() Query {
	var q Query
	if err := q.FromQueryStringQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a NumericRangeQuery. Panics on error.
func (v NumericRangeQuery) ToQuery() Query {
	var q Query
	if err := q.FromNumericRangeQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a TermRangeQuery. Panics on error.
func (v TermRangeQuery) ToQuery() Query {
	var q Query
	if err := q.FromTermRangeQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a DateRangeStringQuery. Panics on error.
func (v DateRangeStringQuery) ToQuery() Query {
	var q Query
	if err := q.FromDateRangeStringQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a BooleanQuery. Panics on error.
func (v BooleanQuery) ToQuery() Query {
	var q Query
	if err := q.FromBooleanQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a ConjunctionQuery. Panics on error.
func (v ConjunctionQuery) ToQuery() Query {
	var q Query
	if err := q.FromConjunctionQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a DisjunctionQuery. Panics on error.
func (v DisjunctionQuery) ToQuery() Query {
	var q Query
	if err := q.FromDisjunctionQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a MatchAllQuery. Panics on error.
func (v MatchAllQuery) ToQuery() Query {
	var q Query
	if err := q.FromMatchAllQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a MatchNoneQuery. Panics on error.
func (v MatchNoneQuery) ToQuery() Query {
	var q Query
	if err := q.FromMatchNoneQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a DocIdQuery. Panics on error.
func (v DocIdQuery) ToQuery() Query {
	var q Query
	if err := q.FromDocIdQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a BoolFieldQuery. Panics on error.
func (v BoolFieldQuery) ToQuery() Query {
	var q Query
	if err := q.FromBoolFieldQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a IPRangeQuery. Panics on error.
func (v IPRangeQuery) ToQuery() Query {
	var q Query
	if err := q.FromIPRangeQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a GeoBoundingBoxQuery. Panics on error.
func (v GeoBoundingBoxQuery) ToQuery() Query {
	var q Query
	if err := q.FromGeoBoundingBoxQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a GeoDistanceQuery. Panics on error.
func (v GeoDistanceQuery) ToQuery() Query {
	var q Query
	if err := q.FromGeoDistanceQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a GeoBoundingPolygonQuery. Panics on error.
func (v GeoBoundingPolygonQuery) ToQuery() Query {
	var q Query
	if err := q.FromGeoBoundingPolygonQuery(v); err != nil {
		panic(err)
	}
	return q
}

// ToQuery creates a Query from a GeoShapeQuery. Panics on error.
func (v GeoShapeQuery) ToQuery() Query {
	var q Query
	if err := q.FromGeoShapeQuery(v); err != nil {
		panic(err)
	}
	return q
}
