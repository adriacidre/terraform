package s3

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/r3labs/terraform/backend"
	"github.com/r3labs/terraform/state"
	"github.com/r3labs/terraform/state/remote"
	"github.com/r3labs/terraform/terraform"
)

const (
	// This will be used as directory name, the odd looking colon is simply to
	// reduce the chance of name conflicts with existing objects.
	keyEnvPrefix = "env:"
)

func (b *Backend) States() ([]string, error) {
	params := &s3.ListObjectsInput{
		Bucket: &b.bucketName,
		Prefix: aws.String(keyEnvPrefix + "/"),
	}

	resp, err := b.s3Client.ListObjects(params)
	if err != nil {
		return nil, err
	}

	envs := []string{backend.DefaultStateName}
	for _, obj := range resp.Contents {
		env := b.keyEnv(*obj.Key)
		if env != "" {
			envs = append(envs, env)
		}
	}

	sort.Strings(envs[1:])
	return envs, nil
}

// extract the env name from the S3 key
func (b *Backend) keyEnv(key string) string {
	// we have 3 parts, the prefix, the env name, and the key name
	parts := strings.SplitN(key, "/", 3)
	if len(parts) < 3 {
		// no env here
		return ""
	}

	// shouldn't happen since we listed by prefix
	if parts[0] != keyEnvPrefix {
		return ""
	}

	// not our key, so don't include it in our listing
	if parts[2] != b.keyName {
		return ""
	}

	return parts[1]
}

func (b *Backend) DeleteState(name string) error {
	if name == backend.DefaultStateName || name == "" {
		return fmt.Errorf("can't delete default state")
	}

	params := &s3.DeleteObjectInput{
		Bucket: &b.bucketName,
		Key:    aws.String(b.path(name)),
	}

	_, err := b.s3Client.DeleteObject(params)
	if err != nil {
		return err
	}

	return nil
}

func (b *Backend) State(name string) (state.State, error) {
	if name == "" {
		return nil, errors.New("missing state name")
	}

	client := &RemoteClient{
		s3Client:             b.s3Client,
		dynClient:            b.dynClient,
		bucketName:           b.bucketName,
		path:                 b.path(name),
		serverSideEncryption: b.serverSideEncryption,
		acl:                  b.acl,
		kmsKeyID:             b.kmsKeyID,
		lockTable:            b.lockTable,
	}

	stateMgr := &remote.State{Client: client}

	//if this isn't the default state name, we need to create the object so
	//it's listed by States.
	if name != backend.DefaultStateName {
		// take a lock on this state while we write it
		lockInfo := state.NewLockInfo()
		lockInfo.Operation = "init"
		lockId, err := client.Lock(lockInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to lock s3 state: %s", err)
		}

		// Local helper function so we can call it multiple places
		lockUnlock := func(parent error) error {
			if err := stateMgr.Unlock(lockId); err != nil {
				return fmt.Errorf(strings.TrimSpace(errStateUnlock), lockId, err)
			}
			return parent
		}

		// Grab the value
		if err := stateMgr.RefreshState(); err != nil {
			err = lockUnlock(err)
			return nil, err
		}

		// If we have no state, we have to create an empty state
		if v := stateMgr.State(); v == nil {
			if err := stateMgr.WriteState(terraform.NewState()); err != nil {
				err = lockUnlock(err)
				return nil, err
			}
			if err := stateMgr.PersistState(); err != nil {
				err = lockUnlock(err)
				return nil, err
			}
		}

		// Unlock, the state should now be initialized
		if err := lockUnlock(nil); err != nil {
			return nil, err
		}

	}

	return stateMgr, nil
}

func (b *Backend) client() *RemoteClient {
	return &RemoteClient{}
}

func (b *Backend) path(name string) string {
	if name == backend.DefaultStateName {
		return b.keyName
	}

	return strings.Join([]string{keyEnvPrefix, name, b.keyName}, "/")
}

const errStateUnlock = `
Error unlocking S3 state. Lock ID: %s

Error: %s

You may have to force-unlock this state in order to use it again.
`
