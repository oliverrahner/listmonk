<template>
  <div>
    <div class="columns mb-6">
      <div class="column is-3">
        <b-field :label="$t('settings.incoming.enable')" data-cy="btn-enable-incoming">
          <b-switch v-model="data['incoming.enabled']" name="incoming.enabled" />
        </b-field>
      </div>
    </div>

    <!-- incoming mailboxes -->
    <template v-if="data['incoming.enabled']">
      <div class="block box" v-for="(item, n) in data['incoming.mailboxes']" :key="n">
        <div class="columns">
          <div class="column is-2">
            <b-field :label="$t('globals.buttons.enabled')">
              <b-switch v-model="item.enabled" name="enabled" :native-value="true" data-cy="btn-enable-incoming-mailbox" />
            </b-field>
            <b-field v-if="data['incoming.mailboxes'].length > 1">
              <a @click.prevent="$utils.confirm(null, () => removeMailbox(n))" href="#" data-cy="btn-delete-incoming-mailbox">
                <b-icon icon="trash-can-outline" />
                {{ $t('globals.buttons.delete') }}
              </a>
            </b-field>
          </div>

          <div class="column" :class="{ disabled: !item.enabled }">
            <div class="columns">
              <div class="column is-3">
                <b-field :label="$t('settings.incoming.type')" label-position="on-border">
                  <b-select v-model="item.type" name="type">
                    <option value="pop">
                      POP
                    </option>
                  </b-select>
                </b-field>
              </div>
              <div class="column is-6">
                <b-field :label="$t('settings.mailserver.host')" label-position="on-border"
                  :message="$t('settings.mailserver.hostHelp')">
                  <b-input v-model="item.host" name="host" placeholder="mail.example.com" :maxlength="200" />
                </b-field>
              </div>
              <div class="column is-3">
                <b-field :label="$t('settings.mailserver.port')" label-position="on-border"
                  :message="$t('settings.mailserver.portHelp')">
                  <b-numberinput v-model="item.port" name="port" type="is-light" controls-position="compact"
                    placeholder="995" min="1" max="65535" />
                </b-field>
              </div>
            </div>

            <div class="columns">
              <div class="column is-3">
                <b-field :label="$t('settings.mailserver.authProtocol')" label-position="on-border">
                  <b-select v-model="item.auth_protocol" name="auth_protocol">
                    <option value="plain">
                      plain
                    </option>
                    <option value="none">
                      none
                    </option>
                  </b-select>
                </b-field>
              </div>
              <div class="column">
                <b-field grouped>
                  <b-field :label="$t('settings.mailserver.username')" label-position="on-border" expanded>
                    <b-input v-model="item.username" :disabled="item.auth_protocol === 'none'" name="username"
                      :custom-class="`incoming-username-${n}`" placeholder="lists@example.com" :maxlength="200" />
                  </b-field>
                  <b-field :label="$t('settings.mailserver.password')" label-position="on-border" expanded
                    :message="$t('settings.mailserver.passwordHelp')">
                    <b-input v-model="item.password" :disabled="item.auth_protocol === 'none'" name="password"
                      type="password" :custom-class="`incoming-password-${n}`"
                      :placeholder="$t('settings.mailserver.passwordHelp')" :maxlength="200" />
                  </b-field>
                </b-field>
              </div>
            </div>

            <div class="columns">
              <div class="column is-6">
                <b-field grouped>
                  <b-field :label="$t('settings.mailserver.tls')" expanded :message="$t('settings.mailserver.tlsHelp')">
                    <b-switch v-model="item.tls_enabled" name="item.tls_enabled" />
                  </b-field>
                  <b-field :label="$t('settings.mailserver.skipTLS')" expanded
                    :message="$t('settings.mailserver.skipTLSHelp')">
                    <b-switch v-model="item.tls_skip_verify" :disabled="!item.tls_enabled"
                      name="item.tls_skip_verify" />
                  </b-field>
                </b-field>
              </div>
              <div class="column" />
              <div class="column is-4">
                <b-field :label="$t('settings.incoming.scanInterval')" expanded label-position="on-border"
                  :message="$t('settings.incoming.scanIntervalHelp')">
                  <b-input v-model="item.scan_interval" name="scan_interval" placeholder="5m" :pattern="regDuration"
                    :maxlength="10" />
                </b-field>
              </div>
            </div>

            <hr />

            <form @submit.prevent="() => doMailboxTest(item, n)">
              <div class="columns">
                <div class="column has-text-right">
                  <b-button v-if="testItem === n" class="is-primary" @click.prevent="() => doMailboxTest(item, n)"
                    :disabled="!isTestEnabled(item)">
                    {{ $t('settings.incoming.testConnection') }}
                  </b-button>
                  <a href="#" v-else class="is-primary" @click.prevent="showTestForm(n)">
                    <b-icon icon="rocket-launch-outline" /> {{ $t('settings.incoming.testConnection') }}
                  </a>
                </div>
              </div>
              <div v-if="errMsg && testItem === n">
                <b-field class="mt-4" type="is-danger">
                  <b-input v-model="errMsg" type="textarea" custom-class="has-text-danger is-size-6" readonly />
                </b-field>
              </div>
            </form>
          </div>
        </div>
      </div>

      <div class="mt-5">
        <b-button @click="addMailbox" icon-left="plus" type="is-primary" data-cy="btn-add-incoming-mailbox">
          {{ $t('settings.incoming.addMailbox') }}
        </b-button>
      </div>
    </template>
  </div>
</template>

<script>
import Vue from 'vue';
import { regDuration } from '../../constants';

export default Vue.extend({
  props: {
    form: {
      type: Object, default: () => { },
    },
  },

  data() {
    return {
      data: this.form,
      regDuration,
      testItem: null,
      errMsg: '',
    };
  },

  methods: {
    addMailbox() {
      this.data['incoming.mailboxes'].push({
        enabled: true,
        uuid: '',
        type: 'pop',
        host: '',
        port: 995,
        auth_protocol: 'plain',
        username: '',
        password: '',
        tls_enabled: true,
        tls_skip_verify: false,
        scan_interval: '5m',
      });
    },

    removeMailbox(i) {
      this.data['incoming.mailboxes'].splice(i, 1);
    },

    showTestForm(n) {
      this.testItem = n;
      this.errMsg = '';

      this.$nextTick(() => {
        const i = document.querySelector(`.incoming-password-${n}`);
        if (i) {
          i.focus();
        }
      });
    },

    isTestEnabled(item) {
      if (!item.host || !item.port) {
        return false;
      }
      if (item.auth_protocol !== 'none' && item.password.includes('•')) {
        return false;
      }

      return true;
    },

    doMailboxTest(item, n) {
      if (!this.isTestEnabled(item)) {
        this.$utils.toast(this.$t('settings.incoming.enterPassword'), 'is-danger');
        this.$nextTick(() => {
          const i = document.querySelector(`.incoming-password-${n}`);
          this.data['incoming.mailboxes'][n].password = '';
          i.focus();
          i.select();
        });
        return;
      }

      this.errMsg = '';
      this.$api.testIncomingMailbox(item).then(() => {
        this.$utils.toast(this.$t('settings.incoming.testSuccess'));
      }).catch((err) => {
        if (err.response?.data?.message) {
          this.errMsg = err.response.data.message;
        }
      });
    },
  },
});
</script>
