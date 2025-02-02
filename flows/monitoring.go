package flows

import (
	"time"

	"github.com/Velocidex/ordereddict"
	artifacts "www.velocidex.com/golang/velociraptor/artifacts"
	config_proto "www.velocidex.com/golang/velociraptor/config/proto"
	"www.velocidex.com/golang/velociraptor/constants"
	crypto_proto "www.velocidex.com/golang/velociraptor/crypto/proto"
	"www.velocidex.com/golang/velociraptor/file_store"
	flows_proto "www.velocidex.com/golang/velociraptor/flows/proto"
	artifact_paths "www.velocidex.com/golang/velociraptor/paths/artifacts"
	"www.velocidex.com/golang/velociraptor/result_sets"
	"www.velocidex.com/golang/velociraptor/services"
	utils "www.velocidex.com/golang/velociraptor/utils"
)

// Receive monitoring messages from the client.
func MonitoringProcessMessage(
	config_obj *config_proto.Config,
	collection_context *flows_proto.ArtifactCollectorContext,
	message *crypto_proto.VeloMessage) error {

	err := FailIfError(config_obj, collection_context, message)
	if err != nil {
		return err
	}

	switch message.RequestId {
	case constants.TransferWellKnownFlowId:
		return appendUploadDataToFile(
			config_obj, collection_context, message)

	}

	response := message.VQLResponse
	if response == nil || response.Query == nil {
		return nil
	}

	// Deobfuscate the response if needed.
	_ = artifacts.Deobfuscate(config_obj, response)

	if response.Query.Name != "" {
		json_response := response.Response
		if json_response == "" {
			json_response = response.JSONLResponse
		}

		// We need to parse each event since it needs to be
		// pushed to the journal, in case a reader is
		// listening to it. FIXME: This is expensive CPU wise,
		// we need to think of a better way to do this.
		rows, err := utils.ParseJsonToDicts([]byte(json_response))
		if err != nil {
			return err
		}

		// Mark the client this came from. Since message.Source
		// is cryptographically trusted, this column may also
		// be trusted.
		for _, row := range rows {
			row.Set("ClientId", message.Source)
		}
		journal, err := services.GetJournal()
		if err != nil {
			return err
		}

		return journal.PushRowsToArtifact(
			config_obj, rows, response.Query.Name,
			message.Source, message.SessionId)
	}

	return nil
}

// Logs from monitoring flow need to be handled especially since they
// are written with a time index.
func flushContextLogsMonitoring(
	config_obj *config_proto.Config,
	collection_context *flows_proto.ArtifactCollectorContext) error {

	// A single packet may have multiple log messages from
	// different artifacts. We cache the writers so we can send
	// the right message to the right log sink.
	writers := make(map[string]result_sets.TimedResultSetWriter)

	// Append logs to messages from previous packets.
	file_store_factory := file_store.GetFileStore(config_obj)
	for _, row := range collection_context.Logs {
		artifact_name := row.Artifact
		if artifact_name == "" {
			artifact_name = "Unknown"
		}

		// Try to get the writer from the cache.
		rs_writer, pres := writers[artifact_name]
		if !pres {
			log_path_manager, err := artifact_paths.NewArtifactLogPathManager(
				config_obj, collection_context.ClientId,
				collection_context.SessionId, artifact_name)
			if err != nil {
				return err
			}

			rs_writer, err = result_sets.NewTimedResultSetWriter(
				file_store_factory, log_path_manager, nil)
			if err != nil {
				return err
			}
			defer rs_writer.Close()

			writers[artifact_name] = rs_writer
		}

		rs_writer.Write(ordereddict.NewDict().
			Set("_ts", int(time.Now().Unix())).
			Set("client_time", int64(row.Timestamp)/1000000).
			Set("message", row.Message))
	}

	// Clear the logs from the flow object.
	collection_context.Logs = nil
	return nil
}
