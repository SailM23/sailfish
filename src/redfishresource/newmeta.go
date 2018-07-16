package domain

import (
    "context"
    "reflect"
    "errors"
    "fmt"
    "sync"
)

type processFn func(context.Context, RedfishResourceProperty, encOpts) (interface{}, error)

type encOpts struct {
	request map[string]interface{}
	process processFn
}

func ProcessPATCH(ctx context.Context, prop RedfishResourceProperty, request map[string]interface{}) (results interface{}, err error) {
	opts := encOpts{
		request: request,
		process: PATCHfn,
	}

	return parseRecursive(ctx, reflect.ValueOf(prop), opts)
}

func ProcessGET(ctx context.Context, prop RedfishResourceProperty) (results interface{}, err error) {
	opts := encOpts{
		request: nil,
		process: GETfn,
	}

	return parseRecursive(ctx, reflect.ValueOf(prop), opts)
}

type Marshaler interface {
    DOMETA(context.Context, encOpts) (interface{}, error)
}

func (rrp RedfishResourceProperty) DOMETA(ctx context.Context, e encOpts) (results interface{}, err error) {
    res, err := e.process(ctx, rrp, e)
    if err == nil {
        return parseRecursive(ctx, reflect.ValueOf(res), e)
    } else {
        return parseRecursive(ctx, reflect.ValueOf(rrp.Value), e)
    }
}

var marshalerType = reflect.TypeOf(new(Marshaler)).Elem()

func parseRecursive(ctx context.Context, val reflect.Value, e encOpts) (interface{}, error) {
    if ! val.IsValid() {
        return nil, errors.New("not a valid type")
    }

    if val.IsValid() && val.Type().Implements(marshalerType) {
        m, ok := val.Interface().(Marshaler)
        if !ok {
            return nil, errors.New("ugh")
        }
        r, err :=  m.DOMETA(ctx, e)
        return r, err
    }

    switch k := val.Kind(); k {
    case reflect.Map:

        var ret reflect.Value
        keyType := val.Type().Key()
        elemType := val.Type().Elem()
        maptype := reflect.MapOf(keyType, elemType)
        ret = reflect.MakeMap(maptype)

        m := sync.Mutex{}
        wg := sync.WaitGroup{}
        for _, k := range val.MapKeys() {
            wg.Add(1)
            go func(k reflect.Value) {
                mapVal := val.MapIndex(k).Interface()
                parsed, _ := parseRecursive(ctx, reflect.ValueOf(mapVal), e)
                m.Lock()
                ret.SetMapIndex(k, reflect.ValueOf(parsed))
                m.Unlock()
                wg.Done()
            }(k)
        }
        wg.Wait()
        return ret.Interface(), nil

    case reflect.Slice:

        var ret reflect.Value
        elemType := val.Type().Elem()
        arraytype := reflect.SliceOf(elemType)
        ret = reflect.MakeSlice(arraytype, val.Len(), val.Cap())

        wg := sync.WaitGroup{}
        for i:=0; i< val.Len(); i++ {
            wg.Add(1)
            go func(k int) {
                sliceVal := val.Index(k)
                parsed, _ := parseRecursive(ctx, reflect.ValueOf(sliceVal.Interface()), e)
                ret.Index(k).Set(reflect.ValueOf(parsed))
                wg.Done()
            }(i)
        }
        wg.Wait()
        return ret.Interface(), nil


    }

    return val.Interface(), nil
}

type NewPropGetter interface {
	PropertyGet(context.Context, RedfishResourceProperty, map[string]interface{}) (interface{}, error)
}
type CompatPropGetter interface {
	PropertyGet(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, map[string]interface{})
}

type NewPropPatcher interface {
	PropertyPatch(context.Context, RedfishResourceProperty, map[string]interface{}, map[string]interface{}) (interface{}, error)
}
type CompatPropPatcher interface {
	PropertyPatch(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, map[string]interface{})
}


func GETfn(ctx context.Context, rrp RedfishResourceProperty, opts encOpts) (interface{}, error) {
	meta_t, ok := rrp.Meta["GET"].(map[string]interface{})
	if !ok {
		return nil, errors.New("No GET")
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		return nil, errors.New("No plugin in GET")
	}

	plugin, err := InstantiatePlugin(PluginType(pluginName))
	if err != nil {
		return nil, errors.New("No plugin named(" + pluginName + ") for GET")
	}

    ContextLogger(ctx, "property_process").Debug("getting property: GET", "value", fmt.Sprintf("%v", rrp.Value))
    if plugin, ok := plugin.(NewPropGetter); ok {
        defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: GET - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
        return plugin.PropertyGet(ctx, rrp, meta_t)
    }
    if plugin, ok := plugin.(CompatPropGetter); ok {
        defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: GET - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
        tempRRP := &RedfishResourceProperty{ Value: rrp.Value, Meta: rrp.Meta }
        plugin.PropertyGet(ctx, nil, tempRRP, meta_t)
        return tempRRP.Value, nil
    }
    return nil, errors.New("foobar")
}


func PATCHfn(ctx context.Context, rrp RedfishResourceProperty, opts encOpts) (interface{}, error) {
	meta_t, ok := rrp.Meta["PATCH"].(map[string]interface{})
	if !ok {
		return nil, errors.New("No PATCH")
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		return nil, errors.New("No plugin in PATCH")
	}

	plugin, err := InstantiatePlugin(PluginType(pluginName))
	if err != nil {
		return nil, errors.New("No plugin named(" + pluginName + ") for PATCH")
	}

    ContextLogger(ctx, "property_process").Debug("getting property: PATCH", "value", fmt.Sprintf("%v", rrp.Value))
    if plugin, ok := plugin.(NewPropPatcher); ok {
        defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
        return plugin.PropertyPatch(ctx, rrp, opts.request, meta_t)
    }
    if plugin, ok := plugin.(CompatPropPatcher); ok {
        defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
        tempRRP := &RedfishResourceProperty{ Value: rrp.Value, Meta: rrp.Meta }
        plugin.PropertyPatch(ctx, nil, tempRRP, meta_t)
        return tempRRP.Value, nil
    }
    return nil, errors.New("foobar")
}
